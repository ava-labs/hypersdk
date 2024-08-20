// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

extern crate proc_macro;

use proc_macro::TokenStream;
use proc_macro2::{Literal, Span};
use quote::{format_ident, quote, ToTokens};
use std::str::FromStr;
use syn::{
    parse::{Parse, ParseStream},
    parse_macro_input, parse_quote, parse_str,
    punctuated::Punctuated,
    spanned::Spanned,
    token, Attribute, Error, Expr, Fields, FnArg, GenericParam, Ident, ItemFn, LitInt, Pat,
    PatIdent, PatType, PatWild, Path, ReturnType, Signature, Token, Type, TypeParam,
    TypeParamBound, TypePath, TypeReference, TypeTuple, Visibility,
};

const CONTEXT_TYPE: &str = "&mut wasmlanche_sdk::Context";

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

    let (input, context_type, first_arg_err) = {
        let mut context_type: Box<Type> = Box::new(parse_str(CONTEXT_TYPE).unwrap());
        let mut input = input;

        let first_arg_err = match input.sig.inputs.first_mut() {
            Some(FnArg::Typed(PatType { ty, .. })) if is_mutable_context_ref(ty) => {
                let types = (context_type.as_mut(), ty.as_mut());

                if let (Type::Reference(context_type), Type::Reference(ty)) = types {
                    context_type.lifetime = ty.lifetime.clone();
                }

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

    let binding_args_props = args_props.iter().map(|arg| FnArg::Typed(arg.clone()));

    let args_names = args_props
        .iter()
        .map(|PatType { pat: name, .. }| quote! {#name});
    let args_names_2 = args_names.clone();

    let name = &input.sig.ident;
    let context_type = type_from_reference(&context_type);

    let external_call = quote! {
        mod private {
            use super::*;
            #[derive(wasmlanche_sdk::borsh::BorshDeserialize)]
            #[borsh(crate = "wasmlanche_sdk::borsh")]
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
            unsafe extern "C" fn #name(args: wasmlanche_sdk::HostPtr) {
                wasmlanche_sdk::register_panic();

                let result = {
                    let args: Args = wasmlanche_sdk::borsh::from_slice(&args).expect("error fetching serialized args");

                    let Args { mut ctx, #(#args_names),* } = args;

                    let result = super::#name(&mut ctx, #(#args_names_2),*);
                    wasmlanche_sdk::borsh::to_vec(&result).expect("error serializing result")
                };

                unsafe { set_call_result(result.as_ptr(), result.len()) };
            }
        }
    };

    let inputs: Punctuated<FnArg, Token![,]> =
        std::iter::once(parse_str("ctx: &wasmlanche_sdk::ExternalCallContext").unwrap())
            .chain(binding_args_props)
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
        let args = wasmlanche_sdk::borsh::to_vec(&(#(#args),*)).expect("error serializing args");
        ctx
            .program()
            .call_function::<#return_type>(#name, &args, ctx.max_units(), ctx.value())
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

    input
        .block
        .stmts
        .insert(0, syn::parse2(external_call).unwrap());

    TokenStream::from(quote! {
        #binding
        #input
    })
}

#[derive(Debug)]
struct KeyPair {
    key_comments: Vec<Attribute>,
    key_vis: Visibility,
    key_type_name: Ident,
    key_fields: Fields,
    value_type: Type,
}

impl Parse for KeyPair {
    fn parse(input: ParseStream) -> syn::Result<Self> {
        let key_comments = input.call(Attribute::parse_outer)?;
        let key_vis = input.parse::<Visibility>()?;
        let key_type_name: Ident = input.parse()?;
        let lookahead = input.lookahead1();

        let key_fields = if lookahead.peek(token::Paren) {
            let fields = input.parse()?;
            Fields::Unnamed(fields)
        } else if lookahead.peek(token::Brace) {
            Fields::Named(input.parse()?)
        } else {
            Fields::Unit
        };

        if let Fields::Named(named) = key_fields {
            return Err(Error::new(
                named.span(),
                "types with named fields are not supported",
            ));
        }

        input.parse::<Token![=>]>()?;
        let value_type = input.parse()?;

        Ok(Self {
            key_comments,
            key_vis,
            key_type_name,
            key_fields,
            value_type,
        })
    }
}

#[proc_macro]
pub fn state_schema(input: TokenStream) -> TokenStream {
    let key_pairs =
        parse_macro_input!(input with Punctuated::<KeyPair, Token![,]>::parse_terminated);
    let result: Result<_, Error> = key_pairs
        .into_iter()
        .enumerate()
        .try_fold(
            quote! {},
            |mut token_stream,
             (
                i,
                KeyPair {
                    key_comments,
                    key_vis,
                    key_type_name,
                    key_fields,
                    value_type,
                },
            )| {
                let i = u8::try_from(i).map_err(|_| {
                    Error::new(
                        key_type_name.span(),
                        "Cannot exceed `u8::MAX + 1` keys in a state-schema",
                    )
                })?;

                token_stream.extend(Some(quote! {
                    #(#key_comments)*
                    #[derive(Copy, Clone, wasmlanche_sdk::bytemuck::NoUninit)]
                    #[bytemuck(crate = "wasmlanche_sdk::bytemuck")]
                    #[repr(C)]
                    #key_vis struct #key_type_name #key_fields;

                    const _: fn() = || {
                        #[doc(hidden)]
                        struct TypeWithoutPadding([u8; 1 + ::core::mem::size_of::<#key_type_name>()]);
                        let _ = ::core::mem::transmute::<wasmlanche_sdk::state::PrefixedKey<#key_type_name>, TypeWithoutPadding>;
                    };

                    unsafe impl wasmlanche_sdk::state::Schema for #key_type_name {
                        type Value = #value_type;

                        fn prefix() -> u8 {
                            #i
                        }
                    }
                }));

                Ok(token_stream)
            },
        );

    match result {
        Ok(token_stream) => token_stream,
        Err(err) => err.to_compile_error(),
    }
    .into()
}

type CommaSeparated<T> = Punctuated<T, Token![,]>;

fn create_n_suffixed_types<S>(ident: &str, n: usize, span: S) -> CommaSeparated<Type>
where
    S: Into<Option<Span>>,
{
    let span = span.into().unwrap_or_else(Span::call_site);

    (0..n)
        .map(|i| Ident::new(&format!("{ident}{i}"), span))
        .map(Path::from)
        .map(|path| TypePath { qself: None, path })
        .map(Type::Path)
        .collect()
}

fn create_n_suffixed_generic_params<S>(
    ident: &str,
    n: usize,
    span: S,
    bounds: Punctuated<TypeParamBound, Token![+]>,
) -> CommaSeparated<GenericParam>
where
    S: Into<Option<Span>>,
{
    let span = span.into().unwrap_or_else(Span::call_site);

    (0..n)
        .map(|i| Ident::new(&format!("{ident}{i}"), span))
        .map(TypeParam::from)
        .map(|param| (bounds.clone(), param))
        .map(|(bounds, param)| TypeParam { bounds, ..param })
        .map(GenericParam::Type)
        .collect()
}

fn create_tuple_of_tuples(a: CommaSeparated<Type>, b: CommaSeparated<Type>) -> Type {
    let paren_token = Default::default();

    let elems = a
        .into_iter()
        .zip(b)
        .map(|(a, b)| TypeTuple {
            paren_token,
            elems: [a, b].into_iter().collect(),
        })
        .map(Type::Tuple)
        .collect();

    Type::Tuple(TypeTuple { paren_token, elems })
}

#[proc_macro]
pub fn impl_to_pairs(inputs: TokenStream) -> TokenStream {
    let mut inputs =
        parse_macro_input!(inputs with Punctuated::<Expr, Token![,]>::parse_terminated).into_iter();

    let n: TokenStream = inputs.next().unwrap().into_token_stream().into();
    let n = parse_macro_input!(n as LitInt)
        .base10_parse::<usize>()
        .unwrap();

    let key_generic_bounds = inputs.next().unwrap().into_token_stream().into();
    let key_generic_bounds = parse_macro_input!(key_generic_bounds with Punctuated::<TypeParamBound, Token![+]>::parse_terminated);
    let keys = create_n_suffixed_types("K", n, None);

    let values = keys
        .clone()
        .into_iter()
        .map(|key| Type::Verbatim(quote! { #key::Value }))
        .collect();

    let tuple_of_key_value_pairs = create_tuple_of_tuples(keys, values);
    let generic_params = create_n_suffixed_generic_params("K", n, None, key_generic_bounds);

    let accessors = (0..n)
        .map(|i| i.to_string())
        .map(|i| <Literal as FromStr>::from_str(&i).unwrap());

    quote! {
        impl<#generic_params> Sealed for #tuple_of_key_value_pairs {}

        impl<#generic_params> IntoPairs for #tuple_of_key_value_pairs {
            fn into_pairs(self) -> impl IntoIterator<Item = Result<(CacheKey, CacheValue), Error>> {
                [
                    #(
                        crate::borsh::to_vec(&self.#accessors.1)
                            .map(|value| (Box::from(to_key(self.#accessors.0).as_ref()), value))
                            .map_err(|_| Error::Serialization)
                    ),*
                ]
            }
        }
    }
    .into()
}

/// Returns whether the type_path represents a Program type.
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
