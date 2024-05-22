extern crate proc_macro;

use proc_macro::TokenStream;
use quote::{format_ident, quote, ToTokens};
use syn::{
    parse_macro_input, parse_str, spanned::Spanned, Fields, FnArg, Ident, ItemEnum, ItemFn, Pat,
    PatType, Path, Type, Visibility,
};

const CONTEXT_TYPE: &str = "wasmlanche_sdk::Context";

/// An attribute procedural macro that makes a function visible to the VM host.
/// It does so by creating an `extern "C" fn` that handles all pointer resolution and deserialization.
/// `#[public]` functions must have `pub` visibility and the first parameter must be of type `Context`.
#[proc_macro_attribute]
pub fn public(_: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as ItemFn);

    let vis_err = if !matches!(input.vis, Visibility::Public(_)) {
        let err = syn::Error::new(
            input.sig.span(),
            "Functions with the `#[public]` attribute must have `pub` visibility.",
        );

        Some(err)
    } else {
        None
    };

    // TODO:
    // prefix with an underscore
    let new_name = {
        let name = &input.sig.ident;
        Ident::new(&format!("{name}_guest"), name.span())
    };

    let (input, user_specified_context_type, first_arg_err) = {
        let mut context_type: Box<Type> = Box::new(parse_str(CONTEXT_TYPE).unwrap());
        let mut input = input;

        let first_arg_err = match input.sig.inputs.first_mut() {
            Some(FnArg::Typed(PatType { ty, .. })) if is_context(ty) => {
                std::mem::swap(&mut context_type, ty);
                None
            }

            arg => {
                let err = match arg {
                Some(FnArg::Typed(PatType { ty, .. })) => {
                    syn::Error::new(
                        ty.span(),
                        format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                    )
                }
                Some(_) => {
                    syn::Error::new(
                        arg.span(),
                        format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`"),
                    )
                }
                None => {
                    syn::Error::new(
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
        FnArg::Receiver(_) => Err(syn::Error::new(
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
            unsafe extern "C" fn #new_name(args: *const u8) {
                let args: Args = unsafe {
                    wasmlanche_sdk::from_host_ptr(args).expect("error fetching serialized args")
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
    // add default attributes
    item_enum.attrs.push(syn::parse_quote! {
         #[derive(Clone, Copy, Debug, Eq, PartialEq, Hash)]
    });
    item_enum.attrs.push(syn::parse_quote! {
         #[repr(u8)]
    });

    let name = &item_enum.ident;
    let variants = &item_enum.variants;

    let to_vec_tokens = generate_to_vec(variants);
    let gen = quote! {
        // generate the original enum definition with attributes
        #item_enum

        // generate the to_vec implementation
        impl #name {
            pub fn to_vec(self) -> Vec<u8> {
                match self {
                    #(#to_vec_tokens),*
                }
            }
        }

        // Generate the Into<key> implementation needed to
        // convert the enum to a Key type.
        impl Into<wasmlanche_sdk::state::Key> for #name {
            fn into(self) -> wasmlanche_sdk::state::Key {
                wasmlanche_sdk::state::Key::new(self.to_vec())
            }
        }
    };

    TokenStream::from(gen)
}

fn generate_to_vec(
    variants: &syn::punctuated::Punctuated<syn::Variant, syn::Token![,]>,
) -> Vec<proc_macro2::TokenStream> {
    variants
        .iter()
        .enumerate()
        .map(|(idx, variant)| {
            let variant_ident = &variant.ident;
            let index = idx as u8;
            match &variant.fields {
                // ex: Point(f64, f64)
                Fields::Unnamed(_) => quote! {
                    Self::#variant_ident(a) => std::iter::once(#index).chain(a.into_iter()).collect()
                },
                // ex: Point
                Fields::Unit => quote! {
                    Self::#variant_ident => vec![#index]
                },
                // ex: Point { x: f64, y: f64 }
                Fields::Named(_) => quote! {
                    Self::#variant_ident { .. } => panic!("named enum fields are not supported"),
                },
            }
        })
        .collect()
}

/// Returns whether the type_path represents a Program type.
fn is_context(type_path: &std::boxed::Box<Type>) -> bool {
    if let Type::Path(type_path) = type_path.as_ref() {
        type_path.path.segments.last() == parse_str::<Path>(CONTEXT_TYPE).unwrap().segments.last()
    } else {
        false
    }
}
