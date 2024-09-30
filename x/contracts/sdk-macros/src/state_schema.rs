// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

use proc_macro2::TokenStream;
use quote::quote;
use syn::{
    parse::{Parse, ParseStream},
    spanned::Spanned,
    token, Attribute, Error, Fields, Ident, Token, Type, Visibility,
};

pub fn impl_state_schema(
    key_pairs: impl IntoIterator<Item = KeyPair>,
) -> Result<TokenStream, Error> {
    key_pairs.into_iter().enumerate().try_fold(
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
                #[derive(Copy, Clone, wasmlanche::bytemuck::NoUninit)]
                #[bytemuck(crate = "wasmlanche::bytemuck")]
                #[repr(C, packed)]
                #key_vis struct #key_type_name #key_fields;

                wasmlanche::prefixed_key_size_check!(wasmlanche::macro_types, #key_type_name);

                unsafe impl wasmlanche::macro_types::Schema for #key_type_name {
                    type Value = #value_type;

                    fn prefix() -> u8 {
                        #i
                    }
                }
            }));

            Ok(token_stream)
        },
    )
}

#[derive(Debug)]
pub struct KeyPair {
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
