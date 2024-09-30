// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use proc_macro2::{Span, TokenStream};
use quote::{format_ident, quote};
use syn::{
    parse::{Parse, ParseStream},
    parse_quote, parse_str,
    punctuated::Punctuated,
    spanned::Spanned,
    Block, Error, FnArg, Generics, Ident, ItemFn, Pat, PatIdent, PatType, PatWild, ReturnType,
    Signature, Token, Type, TypeReference, Visibility,
};

const CONTEXT_TYPE: &str = "&mut wasmlanche::Context";

type CommaSeparated<T> = Punctuated<T, Token![,]>;

pub fn impl_public(public_fn: PublicFn) -> Result<TokenStream, Error> {
    let args_names = public_fn
        .sig
        .other_inputs
        .iter()
        .map(|PatType { pat: name, .. }| quote! {#name});
    let args_names_2 = args_names.clone();

    let name = &public_fn.sig.ident;
    let context_type = type_from_reference(&public_fn.sig.user_defined_context_type);

    let other_inputs = public_fn.sig.other_inputs.iter();

    let external_call = quote! {
        mod private {
            use super::*;
            #[derive(wasmlanche::borsh::BorshDeserialize)]
            #[borsh(crate = "wasmlanche::borsh")]
            struct Args {
                ctx: #context_type,
                #(#other_inputs),*
            }

            #[link(wasm_import_module = "contract")]
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

    let mut binding_fn = public_fn.to_bindings_fn()?;

    let feature_name = "bindings";

    let mut public_fn = public_fn;

    public_fn
        .attrs
        .push(parse_quote! { #[cfg(not(feature = #feature_name))] });
    binding_fn
        .attrs
        .push(parse_quote! { #[cfg(feature = #feature_name)] });

    public_fn
        .block
        .stmts
        .insert(0, syn::parse2(external_call).unwrap());

    let public_fn = ItemFn::from(public_fn);

    let result = quote! {
        #binding_fn
        #public_fn
    };

    Ok(result)
}

pub struct PublicFn {
    attrs: Vec<syn::Attribute>,
    vis: Visibility,
    sig: PublicFnSignature,
    block: Box<Block>,
}

impl Parse for PublicFn {
    fn parse(input: ParseStream) -> syn::Result<Self> {
        let input = ItemFn::parse(input)?;

        let ItemFn {
            attrs,
            vis,
            sig,
            block,
        } = input;

        let vis_err = if !matches!(&vis, Visibility::Public(_)) {
            let err = Error::new(
                sig.span(),
                "Functions with the `#[public]` attribute must have `pub` visibility.",
            );

            Some(err)
        } else {
            None
        };

        let mut inputs = sig.inputs.into_iter();
        let paren_span = sig.paren_token.span.join();

        let (user_defined_context_type, context_input) =
            extract_context_arg(&mut inputs, paren_span);

        let context_input = match (vis_err, context_input) {
            (Some(mut vis_err), Err(context_err)) => {
                vis_err.combine(context_err);
                Err(vis_err)
            }
            (Some(vis_err), Ok(_)) => Err(vis_err),
            (None, context_input) => context_input,
        };

        let other_inputs = map_other_inputs(inputs);

        let (context_input, other_inputs) = match (context_input, other_inputs) {
            (Err(mut vis_and_first), Err(rest)) => {
                vis_and_first.combine(rest);
                Err(vis_and_first)
            }
            (Ok(_), Err(e)) | (Err(e), Ok(_)) => Err(e),
            (Ok(context_input), Ok(other_inputs)) => Ok((context_input, other_inputs)),
        }?;

        let fn_token = sig.fn_token;

        let sig = PublicFnSignature {
            fn_token,
            ident: sig.ident,
            generics: sig.generics,
            paren_token: sig.paren_token,
            user_defined_context_type,
            context_input,
            other_inputs,
            output: sig.output,
        };

        Ok(Self {
            attrs,
            vis,
            sig,
            block,
        })
    }
}

impl From<PublicFn> for ItemFn {
    fn from(public_fn: PublicFn) -> Self {
        let PublicFn {
            attrs,
            vis,
            sig,
            block,
        } = public_fn;

        let sig = sig.into();

        Self {
            attrs,
            vis,
            sig,
            block,
        }
    }
}

#[derive(Clone)]
struct PublicFnSignature {
    ident: Ident,
    generics: Generics,
    fn_token: Token![fn],
    paren_token: syn::token::Paren,
    user_defined_context_type: Box<Type>,
    context_input: ContextArg,
    other_inputs: CommaSeparated<PatType>,
    output: ReturnType,
}

impl From<PublicFnSignature> for Signature {
    fn from(value: PublicFnSignature) -> Self {
        let PublicFnSignature {
            fn_token,
            ident,
            generics,
            paren_token,
            user_defined_context_type: _,
            context_input,
            other_inputs,
            output,
        } = value;

        let inputs = std::iter::once(FnArg::from(context_input))
            .chain(other_inputs.into_iter().map(FnArg::Typed))
            .collect();

        Self {
            constness: None,
            asyncness: None,
            unsafety: None,
            abi: None,
            fn_token,
            ident,
            generics,
            paren_token,
            inputs,
            variadic: None,
            output,
        }
    }
}

#[derive(Clone)]
struct ContextArg {
    pat: Box<Pat>,
    ty: Box<Type>,
    colon_token: Token![:],
}

impl From<ContextArg> for FnArg {
    fn from(value: ContextArg) -> Self {
        let attrs = Vec::new();
        let ContextArg {
            pat,
            ty,
            colon_token,
        } = value;

        FnArg::Typed(PatType {
            attrs,
            pat,
            ty,
            colon_token,
        })
    }
}

impl PublicFn {
    fn to_bindings_fn(&self) -> Result<ItemFn, Error> {
        let Self {
            attrs,
            vis,
            sig,
            block: _,
        } = self;

        let args = sig.other_inputs.iter().map(|PatType { pat, .. }| pat);
        let name = sig.ident.to_string();

        let block = Box::new(parse_quote! {{
            let args = wasmlanche::borsh::to_vec(&(#(#args,)*)).expect("error serializing args");
            ctx
                .call_function(#name, &args)
                .expect("calling the external contract failed")
        }});

        let ty = type_from_reference(&sig.user_defined_context_type);

        let ty = parse_quote!(wasmlanche::ExternalCallContext<'_, #ty>);

        let context_input = ContextArg {
            pat: parse_quote! { ctx },
            colon_token: self.sig.context_input.colon_token,
            ty,
        };

        let sig = Signature::from(PublicFnSignature {
            context_input,
            ..sig.clone()
        });

        let item_fn = ItemFn {
            attrs: attrs.clone(),
            vis: vis.clone(),
            sig,
            block,
        };

        Ok(item_fn)
    }
}

fn extract_context_arg<Iter>(
    inputs: &mut Iter,
    paren_span: Span,
) -> (Box<Type>, Result<ContextArg, Error>)
where
    Iter: Iterator<Item = FnArg>,
{
    let mut user_defined_context_type: Box<Type> = Box::new(parse_str(CONTEXT_TYPE)).unwrap();

    let first_arg = inputs.next();

    let Some(first_arg) = first_arg else {
        let err = Error::new(
                    paren_span,
                    format!("Functions with the `#[public]` attribute must have at least one parameter and the first parameter must be of type `{CONTEXT_TYPE}`"),
                );

        return (user_defined_context_type, Err(err));
    };

    let result = match first_arg {
        FnArg::Typed(PatType {
            mut ty,
            pat,
            colon_token,
            ..
        }) if is_mutable_context_ref(&ty) => {
            let types = (user_defined_context_type.as_mut(), ty.as_mut());

            if let (Type::Reference(context_type), Type::Reference(ty)) = types {
                context_type.lifetime = ty.lifetime.clone();
            }

            // swap the fully qualified `Context` with the user-defined `Context`
            // for better compiler errors
            std::mem::swap(types.0, types.1);

            Ok(ContextArg {
                pat,
                ty,
                colon_token,
            })
        }

        first_arg => {
            let message = format!("The first paramter of a function with the `#[public]` attribute must be of type `{CONTEXT_TYPE}`");

            let span = match first_arg {
                FnArg::Typed(PatType { ty, .. }) => ty.span(),
                _ => first_arg.span(),
            };

            Err(Error::new(span, message))
        }
    };

    (user_defined_context_type, result)
}

fn map_other_inputs(inputs: impl Iterator<Item = FnArg>) -> Result<CommaSeparated<PatType>, Error> {
    let other_inputs = inputs.enumerate().map(|(i, fn_arg)| match fn_arg {
        FnArg::Receiver(_) => Err(Error::new(
            fn_arg.span(),
            "Functions with the `#[public]` attribute cannot have a `self` parameter.",
        )),
        FnArg::Typed(mut pat_type) => {
            let pat = pat_type.pat;
            pat_type.pat = match *pat {
                // replace wildcards with generated identifiers
                Pat::Wild(PatWild { attrs, .. }) => Pat::Ident(PatIdent {
                    attrs,
                    by_ref: None,
                    mutability: None,
                    ident: format_ident!("arg{}", i),
                    subpat: None,
                }),
                pat => pat,
            }
            .into();

            Ok(pat_type)
        }
    });

    // this isn't the same thing as a `try_fold` because we don't exit early on an error
    #[allow(clippy::manual_try_fold)]
    other_inputs.fold(Ok(Punctuated::new()), |result, param| {
        match (result, param) {
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
        }
    })
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
