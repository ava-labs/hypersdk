extern crate proc_macro;

use proc_macro::TokenStream;
use quote::{format_ident, quote, ToTokens};
use syn::{
    parse_macro_input, parse_quote, parse_str, spanned::Spanned, Error, Fields, FnArg, Ident,
    ItemEnum, ItemFn, Pat, PatType, Path, Type, Visibility,
};

const CONTEXT_TYPE: &str = "wasmlanche_sdk::Context";

/// An attribute procedural macro that makes a function visible to the VM host.
/// It does so by creating an `extern "C" fn` that handles all pointer resolution and deserialization.
/// `#[public]` functions must have `pub` visibility and the first parameter must be of type `Context`.
#[proc_macro_attribute]
pub fn public(_: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as ItemFn);

    let vis_err = if !matches!(input.vis, Visibility::Public(_)) {
        let err = Error::new(
            input.sig.span(),
            "Functions with the `#[public]` attribute must have `pub` visibility.",
        );

        Some(err)
    } else {
        None
    };

    let (input, user_specified_context_type, first_arg_err) = {
        let mut context_type: Box<Type> = Box::new(parse_str(CONTEXT_TYPE).unwrap());
        let mut input = input;

        let first_arg_err = match input.sig.inputs.first_mut() {
            Some(FnArg::Typed(PatType { ty, .. })) if is_context(ty) => {
                let types = (context_type.as_mut(), ty.as_mut());

                if let (Type::Path(context_type), Type::Path(ty)) = types {
                    let args = [context_type, ty].map(|type_path| {
                        type_path
                            .path
                            .segments
                            .last_mut()
                            .map(|segment| &mut segment.arguments)
                    });

                    if let [Some(context_type_args), Some(ty_args)] = args {
                        *context_type_args = ty_args.clone();
                    }
                }

                std::mem::swap(&mut context_type, ty);
                None
            }

            arg => {
                let err = match arg {
                Some(FnArg::Typed(PatType { ty, .. })) => {
                    Error::new(
                        ty.span(),
                        format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                    )
                }
                Some(_) => {
                    Error::new(
                        arg.span(),
                        format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                    )
                }
                None => {
                    Error::new(
                        input.sig.paren_token.span.join(),
                        format!("Functions with the `#[public]` attribute must have at least one parameter and the first parameter must be of type `{CONTEXT_TYPE}`"),
                    )
                }
            };

                Some(err)
            }
        };

        (input, context_type, first_arg_err)
    };

    let input_types_iter = input.sig.inputs.iter().skip(1).map(|fn_arg| match fn_arg {
        FnArg::Receiver(_) => Err(Error::new(
            fn_arg.span(),
            "Functions with the `#[public]` attribute cannot have a `self` parameter.",
        )),
        FnArg::Typed(PatType { ty, .. }) => Ok(ty.clone()),
    });

    let arg_props = std::iter::once(Ok(user_specified_context_type))
        .chain(input_types_iter)
        .enumerate()
        .map(|(i, ty)| {
            ty.map(|ty| PatType {
                attrs: vec![],
                pat: Box::new(Pat::Verbatim(
                    format_ident!("param_{}", i).into_token_stream(),
                )),
                colon_token: Default::default(),
                ty,
            })
        });

    let result = match (vis_err, first_arg_err) {
        (None, None) => Ok(vec![]),
        (Some(err), None) | (None, Some(err)) => Err(err),
        (Some(mut vis_err), Some(first_arg_err)) => {
            vis_err.combine(first_arg_err);
            Err(vis_err)
        }
    };

    let arg_props_or_err = arg_props.fold(result, |result, param| match (result, param) {
        // ignore Ok or first error encountered
        (Err(errors), Ok(_)) | (Ok(_), Err(errors)) => Err(errors),
        // combine errors
        (Err(mut errors), Err(e)) => {
            errors.combine(e);
            Err(errors)
        }
        // collect results
        (Ok(mut names), Ok(name)) => {
            names.push(name);
            Ok(names)
        }
    });

    let args_props = match arg_props_or_err {
        Ok(param_names) => param_names,
        Err(errors) => return errors.to_compile_error().into(),
    };

    let converted_params = args_props.iter().map(|PatType { pat: name, .. }| {
        quote! {
           args.#name
        }
    });

    let name = &input.sig.ident;

    let external_call = quote! {
        mod private {
            use super::*;
            #[derive(borsh::BorshDeserialize)]
            struct Args {
                #(#args_props),*
            }

            #[link(wasm_import_module = "program")]
            extern "C" {
                #[link_name = "set_call_result"]
                fn set_call_result(ptr: *const u8, len: usize);
            }

            #[no_mangle]
            unsafe extern "C" fn #name(args: wasmlanche_sdk::HostPtr) {
                let args: Args = unsafe {
                    borsh::from_slice(&args).expect("error fetching serialized args")
                };

                let result = super::#name(#(#converted_params),*);
                let result = borsh::to_vec(&result).expect("error serializing result");
                unsafe { set_call_result(result.as_ptr(), result.len()) };
            }
        }
    };

    let mut input = input;

    input
        .block
        .stmts
        .insert(0, syn::parse2(external_call).unwrap());

    TokenStream::from(quote! { #input })
}

/// This macro assists in defining the schema for a program's state.  A user can
/// simply define an enum with the desired state keys and the macro will
/// generate the necessary code to convert the enum to a byte vector.
/// The enum will automatically derive the Copy and Clone traits. As well as the
/// repr(u8) attribute.
///
/// Note: The enum variants with named fields are not supported.
#[proc_macro_attribute]
pub fn state_keys(_attr: TokenStream, item: TokenStream) -> TokenStream {
    let mut item_enum = parse_macro_input!(item as ItemEnum);

    if !matches!(item_enum.vis, Visibility::Public(_)) {
        return Error::new(
            item_enum.span(),
            "`enum`s with the `#[state_keys]` attribute must have `pub` visibility.",
        )
        .to_compile_error()
        .into();
    }

    // add default attributes
    item_enum.attrs.push(parse_quote! {
         #[derive(Clone, Copy, Debug, Eq, PartialEq, Hash)]
    });

    let name = &item_enum.ident;
    let variants = &item_enum.variants;

    let match_arms: Result<Vec<_>, _> = variants
        .iter()
        .enumerate()
        .map(|(idx, variant)| {
            let variant_ident = &variant.ident;
            let idx = idx as u8;

            match &variant.fields {
                // TODO:
                // use bytemuck to represent the raw bytes of the key
                // and figure out way to enforce backwards compatibility
                Fields::Unnamed(fields) => {
                    let fields = &fields.unnamed;

                    let fields = fields
                        .iter()
                        .enumerate()
                        .map(|(i, field)| Ident::new(&format!("field_{i}"), field.span()));
                    let fields_2 = fields.clone();
                    let fields_3 = fields.clone();

                    Ok(quote! {
                        Self::#variant_ident(#(#fields),*) => {
                            let len = 1 + #(#fields_2.as_ref().len())+*;
                            let len = u32::try_from(len).map_err(|_| std::io::ErrorKind::InvalidData)?;
                            writer.write_all(&len.to_le_bytes())?;
                            writer.write_all(&[#idx])?;
                            #(
                                writer.write_all(#fields_3.as_ref())?;
                            )*
                        }
                    })
                }

                Fields::Unit => Ok(quote! {
                    Self::#variant_ident => {
                        let len = 1u32;
                        writer.write_all(&len.to_le_bytes())?;
                        writer.write_all(&[#idx])?;
                    }
                }),

                Fields::Named(_) => Err(Error::new(
                    variant_ident.span(),
                    "enums with named fields are not supported".to_string(),
                )
                .into_compile_error()),
            }
        })
        .collect();

    let match_arms = match match_arms {
        Ok(match_arms) => match_arms,
        Err(err) => return err.into(),
    };

    let trait_implementation_body = if !variants.is_empty() {
        quote! { match self { #(#match_arms),* } }
    } else {
        quote! {}
    };

    quote! {
        #item_enum
        impl borsh::BorshSerialize for #name {
            fn serialize<W: borsh::io::Write>(&self, writer: &mut W) -> borsh::io::Result<()> {
                #trait_implementation_body
                Ok(())
            }
        }
        impl wasmlanche_sdk::state::Key for #name {}
    }
    .into()
}

/// Returns whether the type_path represents a Program type.
fn is_context(type_path: &Type) -> bool {
    if let Type::Path(type_path) = type_path {
        let context_path = parse_str::<Path>(CONTEXT_TYPE).unwrap();
        let context_ident = context_path.segments.last().map(|segment| &segment.ident);
        type_path.path.segments.last().map(|segment| &segment.ident) == context_ident
    } else {
        false
    }
}
