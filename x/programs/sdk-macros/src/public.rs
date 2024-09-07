// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use proc_macro2::TokenStream;
use quote::{format_ident, quote};
use syn::{
    parse_quote, parse_str, punctuated::Punctuated, spanned::Spanned, Error, FnArg, ItemFn, Pat,
    PatIdent, PatType, PatWild, ReturnType, Signature, Token, Type, TypeReference, Visibility,
};

const CONTEXT_TYPE: &str = "&mut wasmlanche::Context";

pub fn impl_public(input: ItemFn) -> Result<TokenStream, Error> {
    let vis_err = if !matches!(input.vis, Visibility::Public(_)) {
        let err = Error::new(
            input.sig.span(),
            "Functions with the `#[public]` attribute must have `pub` visibility.",
        );

        Some(err)
    } else {
        None
    };

    let (input, user_defined_context_type, first_arg_err) = {
        let mut context_type: Box<Type> = Box::new(parse_str(CONTEXT_TYPE)?);
        let mut input = input;

        let first_arg_err = match input.sig.inputs.first_mut() {
            Some(FnArg::Typed(PatType { ty, .. })) if is_mutable_context_ref(ty) => {
                let types = (context_type.as_mut(), ty.as_mut());

                if let (Type::Reference(context_type), Type::Reference(ty)) = types {
                    context_type.lifetime = ty.lifetime.clone();
                }

                // swap the fully qualified `Context` with the user-defined `Context`
                std::mem::swap(&mut context_type, ty);
                None
            }

            first_arg => {
                let err = match first_arg {
                    Some(fn_arg) => {
                        let message = format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`");

                        let span = match fn_arg {
                            FnArg::Typed(PatType { ty, .. }) => ty.span(),
                            _ => fn_arg.span(),
                        };

                        Error::new(span, message)
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

    let arg_props = input
        .sig
        .inputs
        .iter()
        .skip(1)
        .enumerate()
        .map(|(i, fn_arg)| match fn_arg {
            FnArg::Receiver(_) => Err(Error::new(
                fn_arg.span(),
                "Functions with the `#[public]` attribute cannot have a `self` parameter.",
            )),
            FnArg::Typed(pat_type) => {
                let pat = match pat_type.pat.as_ref() {
                    Pat::Wild(PatWild { attrs, .. }) => Pat::Ident(PatIdent {
                        attrs: attrs.clone(),
                        by_ref: None,
                        mutability: None,
                        ident: format_ident!("arg{}", i),
                        subpat: None,
                    }),
                    pat => pat.clone(),
                }
                .into();

                let pat_type = PatType {
                    pat,
                    ..pat_type.clone()
                };

                Ok(pat_type)
            }
        });

    let result = match (vis_err, first_arg_err) {
        (None, None) => Ok(vec![]),
        (Some(err), None) | (None, Some(err)) => Err(err),
        (Some(mut vis_err), Some(first_arg_err)) => {
            vis_err.combine(first_arg_err);
            Err(vis_err)
        }
    };

    let args_props = arg_props.fold(result, |result, param| match (result, param) {
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
    })?;

    let binding_args_props = args_props.iter().map(|arg| FnArg::Typed(arg.clone()));

    let args_names = args_props
        .iter()
        .map(|PatType { pat: name, .. }| quote! {#name});
    let args_names_2 = args_names.clone();

    let name = &input.sig.ident;
    let context_type = type_from_reference(&user_defined_context_type);

    let external_call = quote! {
        mod private {
            use super::*;
            #[derive(wasmlanche::borsh::BorshDeserialize)]
            #[borsh(crate = "wasmlanche::borsh")]
            struct Args {
                ctx: #context_type,
                #(#args_props),*
            }

            #[link(wasm_import_module = "program")]
            extern "C" {
                #[link_name = "set_call_result"]
                fn set_call_result(ptr: *const u8, len: usize);
            }

            #[no_mangle]
            unsafe extern "C-unwind" fn #name(args: wasmlanche::HostPtr) {
                wasmlanche::register_panic();

                let result = {
                    let args: Args = wasmlanche::borsh::from_slice(&args).expect("error fetching serialized args");

                    let Args { mut ctx, #(#args_names),* } = args;

                    let result = super::#name(&mut ctx, #(#args_names_2),*);
                    wasmlanche::borsh::to_vec(&result).expect("error serializing result")
                };

                unsafe { set_call_result(result.as_ptr(), result.len()) };
            }
        }
    };

    let inputs: Punctuated<FnArg, Token![,]> = std::iter::once(parse_quote!(ctx: &#context_type))
        .chain(binding_args_props)
        .collect();
    let args = inputs.iter().skip(1).map(|arg| match arg {
        FnArg::Typed(PatType { pat, .. }) => pat,
        // inputs already validated above
        _ => unreachable!(),
    });
    let name = name.to_string();

    let return_type = match &input.sig.output {
        ReturnType::Type(_, ty) => ty.as_ref().clone(),
        ReturnType::Default => parse_quote!(()),
    };

    let block = Box::new(parse_quote! {{
        let args = wasmlanche::borsh::to_vec(&(#(#args),*)).expect("error serializing args");
        ctx
            .call_function::<#return_type>(#name, &args)
            .expect("calling the external contract failed")
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

    input
        .block
        .stmts
        .insert(0, syn::parse2(external_call).unwrap());

    let result = quote! {
        #binding
        #input
    };

    Ok(result)
}

/// Returns whether the type_path represents a mutable context ref type.
fn is_mutable_context_ref(type_path: &Type) -> bool {
    let Type::Reference(TypeReference {
        mutability: Some(mutability),
        elem,
        ..
    }) = type_path
    else {
        return false;
    };

    // span is ignored in the comparison
    if mutability != &Token![mut](mutability.span()) {
        return false;
    }

    if let Type::Path(type_path) = elem.as_ref() {
        let context_path = parse_str::<TypeReference>(CONTEXT_TYPE).unwrap();
        let Type::Path(context_path) = context_path.elem.as_ref() else {
            return false;
        };

        let context_ident = context_path
            .path
            .segments
            .last()
            .map(|segment| &segment.ident);
        type_path.path.segments.last().map(|segment| &segment.ident) == context_ident
    } else {
        false
    }
}

fn type_from_reference(type_path: &Type) -> &Type {
    if let Type::Reference(TypeReference { elem, .. }) = type_path {
        elem.as_ref()
    } else {
        type_path
    }
}
