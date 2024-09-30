// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use proc_macro2::{Literal, Span, TokenStream};
use quote::{quote, ToTokens};
use syn::{
    parse::Parser, parse2 as parse, punctuated::Punctuated, Error, Expr, GenericParam, Ident,
    LitInt, Path, Token, Type, TypeParam, TypeParamBound, TypePath, TypeTuple,
};

pub fn to_pairs(inputs: impl IntoIterator<Item = Expr>) -> Result<TokenStream, Error> {
    let mut inputs = inputs.into_iter();
    let n = {
        let Some(n) = inputs.next().map(Expr::into_token_stream) else {
            return Err(Error::new(
                Span::call_site(),
                "first argument must be a usize",
            ));
        };
        parse::<LitInt>(n)?.base10_parse::<usize>()?
    };

    let keys = create_n_suffixed_types("K", n, None);
    let values = keys
        .clone()
        .into_iter()
        .map(|key| Type::Verbatim(quote! { #key::Value }))
        .collect();

    let tuple_of_key_value_pairs = create_tuple_of_tuples(keys, values);

    let Some(key_generic_bounds) = inputs.next().map(Expr::into_token_stream) else {
        return Err(Error::new(
            Span::call_site(),
            "second argument must be a generic bound",
        ));
    };

    let key_generic_bounds = Parser::parse2(
        Punctuated::<TypeParamBound, Token![+]>::parse_terminated,
        key_generic_bounds,
    )?;

    let generic_params = create_n_suffixed_generic_params("K", n, None, key_generic_bounds);

    let accessors = (0..n).map(Literal::usize_unsuffixed);

    let result = quote! {
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
    };

    Ok(result)
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
