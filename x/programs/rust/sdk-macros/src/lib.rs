extern crate proc_macro;

use proc_macro::TokenStream;
use proc_macro2::Span;
use quote::quote;
use syn::{
    parse_macro_input, parse_quote, parse_str, punctuated::Punctuated, spanned::Spanned, Error,
    Fields, FnArg, Ident, ItemEnum, ItemFn, PatType, Path, ReturnType, Signature, Token, Type,
    Visibility,
};
use syn::{Pat, PatIdent};

const CONTEXT_TYPE: &str = "wasmlanche_sdk::Context";

/// An attribute procedural macro that makes a function visible to the VM host.
/// It does so by creating an `extern "C" fn` that handles all pointer resolution and deserialization.
/// `#[public]` functions must have `pub` visibility and the first parameter must be of type `Context`.
#[proc_macro_attribute]
pub fn public(_: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as ItemFn);

    let vis_err = if !matches!(input.vis, Visibility::Public(_)) {
        Err(Error::new(
            input.sig.span(),
            "Functions with the `#[public]` attribute must have `pub` visibility.",
        ))
    } else {
        Ok(())
    };

    let mut input = input;
    let user_specified_context_type = match input.sig.inputs.first_mut() {
        Some(FnArg::Typed(pat_type)) if is_context(&pat_type.ty) => {
            if matches!(*pat_type.pat, Pat::Wild(_)) {
                pat_type.pat = Box::new(Pat::Ident(PatIdent {
                    attrs: vec![],
                    by_ref: None,
                    mutability: None,
                    ident: Ident::new("_ctx", Span::call_site()),
                    subpat: None,
                }))
            }
            Ok(pat_type.clone())
        }

        arg => {
            match arg {
                Some(FnArg::Typed(PatType { ty, .. })) => Err(Error::new(
                    ty.span(),
                    format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                )),
                Some(_) => Err(Error::new(
                    arg.span(),
                    format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                )),
                None => Err(Error::new(
                    input.sig.paren_token.span.join(),
                    format!("Functions with the `#[public]` attribute must have at least one parameter and the first parameter must be of type `{CONTEXT_TYPE}`"),
                ))
            }
        }
    };

    let input_types_iter = input.sig.inputs.iter().skip(1).map(|fn_arg| match fn_arg {
        FnArg::Receiver(_) => Err(Error::new(
            fn_arg.span(),
            "Functions with the `#[public]` attribute cannot have a `self` parameter.",
        )),
        FnArg::Typed(pat) => Ok(pat.clone()),
    });

    // TODO borrow user_specified_context_type
    let result = match (vis_err, user_specified_context_type.clone()) {
        (Ok(_), Ok(_)) => Ok(vec![]),
        (Err(err), Ok(_)) | (Ok(_), Err(err)) => Err(err),
        (Err(mut vis_err), Err(first_arg_err)) => {
            vis_err.combine(first_arg_err);
            Err(vis_err)
        }
    };

    let arg_props = std::iter::once(user_specified_context_type).chain(input_types_iter);

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

    let mut binding_args_props = args_props.clone();
    let context_arg_prop = binding_args_props
        .first_mut()
        .expect("first arg context has been checked above");
    let Pat::Ident(PatIdent {
        ident: context_ident,
        ..
    }) = *context_arg_prop.pat.clone()
    else {
        unreachable!()
    };
    context_arg_prop.ty = Box::new(parse_quote!(&wasmlanche_sdk::ExternalCallContext));

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
                wasmlanche_sdk::register_panic();

                let args: Args = unsafe {
                    borsh::from_slice(&args).expect("error fetching serialized args")
                };

                let result = super::#name(#(#converted_params),*);
                let result = borsh::to_vec(&result).expect("error serializing result");
                unsafe { set_call_result(result.as_ptr(), result.len()) };
            }
        }
    };

    let inputs: Punctuated<FnArg, Token![,]> = binding_args_props
        .into_iter()
        .map(|arg| FnArg::Typed(arg))
        .collect();
    let args = inputs.iter().skip(1).map(|arg| match arg {
        FnArg::Typed(PatType { pat, .. }) => pat,
        _ => unreachable!(),
    });
    let name = name.to_string();

    let return_type = match &input.sig.output {
        ReturnType::Type(_, ty) => ty.as_ref().clone(),
        ReturnType::Default => parse_quote!(()),
    };

    let block = Box::new(parse_quote! {{
        let args = borsh::to_vec(&(#(#args),*)).expect("error serializing args");
        #context_ident
            .program()
            .call_function::<#return_type>(#name, &args, #context_ident.max_units(), #context_ident.value())
            .expect("calling the external program failed")
    }});

    let sig = Signature {
        inputs,
        ..input.sig.clone()
    };

    let mut binding = ItemFn {
        sig,
        block,
        ..input.clone()
    };

    let mut input = input;

    let feature_name = "bindings";
    input
        .attrs
        .push(parse_quote! { #[cfg(not(feature = #feature_name))] });
    binding
        .attrs
        .push(parse_quote! { #[cfg(feature = #feature_name)] });

    let parsed = syn::parse2(external_call);
    input.block.stmts.insert(0, parsed.unwrap());

    TokenStream::from(quote! {
        #binding
        #input
    })
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

    const MAX_VARIANTS: usize = u8::MAX as usize + 1;

    if variants.len() > MAX_VARIANTS {
        return Error::new(
            variants[MAX_VARIANTS].span(),
            "Cannot exceed `u8::MAX` variants",
        )
        .into_compile_error()
        .into();
    }

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

                    Ok(quote! {
                        Self::#variant_ident(#(#fields),*) => {
                            borsh::to_vec(&(#idx, #(#fields_2),*))?.serialize(writer)?;
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
        unsafe impl wasmlanche_sdk::state::Key for #name {}
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
