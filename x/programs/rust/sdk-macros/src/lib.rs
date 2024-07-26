extern crate proc_macro;

use proc_macro::TokenStream;
use proc_macro2::{Literal, Span};
use quote::{quote, ToTokens};
use std::str::FromStr;
use syn::{
    parse::{Parse, ParseStream},
    parse_macro_input, parse_quote, parse_str,
    punctuated::Punctuated,
    spanned::Spanned,
    token, Attribute, Error, Expr, Fields, FnArg, GenericParam, Ident, ItemEnum, ItemFn, LitInt,
    PatType, Path, ReturnType, Signature, Token, Type, TypeParam, TypeParamBound, TypePath,
    TypeReference, TypeTuple, Visibility,
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

                    let args = [context_type, ty].map(|type_path| {
                        if let Type::Path(type_path) = type_path.elem.as_mut() {
                            type_path
                                .path
                                .segments
                                .last_mut()
                                .map(|segment| &mut segment.arguments)
                        } else {
                            None
                        }
                    });

                    if let [Some(context_type_args), Some(ty_args)] = args {
                        *context_type_args = ty_args.clone();
                    }
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

    let arg_props = input.sig.inputs.iter().skip(1).map(|fn_arg| match fn_arg {
        FnArg::Receiver(_) => Err(Error::new(
            fn_arg.span(),
            "Functions with the `#[public]` attribute cannot have a `self` parameter.",
        )),
        FnArg::Typed(pat_type) => Ok(pat_type),
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

    let binding_args_props = args_props.iter().map(|&arg| FnArg::Typed(arg.clone()));

    let args_names = args_props
        .iter()
        .map(|PatType { pat: name, .. }| quote! {#name});
    let args_names_2 = args_names.clone();

    let name = &input.sig.ident;
    let context_type = type_from_reference(&context_type);

    let external_call = quote! {
        mod private {
            use super::*;
            #[derive(borsh::BorshDeserialize)]
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

                let args: Args = borsh::from_slice(&args).expect("error fetching serialized args");

                // using converted_params twice here (need to clone)
                // would help to give a specific name to context
                let Args { mut ctx, #(#args_names),* } = args;

                let result = super::#name(&mut ctx, #(#args_names_2),*);
                let result = borsh::to_vec(&result).expect("error serializing result");

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
        let args = borsh::to_vec(&(#(#args),*)).expect("error serializing args");
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
         #[derive(Clone, Copy, Debug, Eq, PartialEq, Hash, wasmlanche_sdk::bytemuck::NoUninit)]
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

#[derive(Debug)]
struct KeyPair {
    key_comments: Vec<Attribute>,
    key_vis: Visibility,
    key_type_name: Ident,
    key_fields: Fields,
    value_type: Ident,
}

impl Parse for KeyPair {
    fn parse(input: ParseStream) -> syn::Result<Self> {
        let key_comments = input.call(Attribute::parse_outer)?;
        let key_vis = input.parse::<Visibility>()?;
        let key_type_name: Ident = input.parse()?;
        let lookahead = input.lookahead1();

        // TODO: fail on named fields
        let key_fields = if lookahead.peek(token::Paren) {
            let fields = input.parse()?;
            Fields::Unnamed(fields)
        } else {
            Fields::Unit
        };

        input.parse::<Token![=>]>()?;
        let value_type: Ident = input.parse()?;

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
    // parse out the key-pairs
    let key_pairs =
        parse_macro_input!(input with Punctuated::<KeyPair, Token![,]>::parse_terminated);
    key_pairs
        .into_iter()
        .enumerate()
        .map(|(i, val)| (i as u8, val))
        .fold(
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
                token_stream.extend(Some(quote! {
                    #(#key_comments)*
                    #[derive(Copy, Clone, bytemuck::NoUninit)]
                    #[repr(C)]
                    #key_vis struct #key_type_name #key_fields;

                    unsafe impl wasmlanche_sdk::state::Schema for #key_type_name {
                        type Value = #value_type;

                        fn prefix() -> u8 {
                            #i
                        }
                    }
                }));

                token_stream
            },
        )
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
        .map(|ident| Path::from(ident))
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
        .zip(b.into_iter())
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

    // let value_generic_bounds = inputs.next().unwrap().into_token_stream().into();
    // let value_generic_bounds = parse_macro_input!(value_generic_bounds with Punctuated::<TypeParamBound, Token![+]>::parse_terminated);

    let keys = create_n_suffixed_types("K", n, None);
    // let values = create_n_suffixed_types("V", n, None);
    let values = keys
        .clone()
        .into_iter()
        .map(|key| Type::Verbatim(quote! { #key::Value }))
        .collect();

    let tuple_of_key_value_pairs = create_tuple_of_tuples(keys, values);

    let key_generic_params = create_n_suffixed_generic_params("K", n, None, key_generic_bounds);
    // let value_generic_params = create_n_suffixed_generic_params("V", n, None, value_generic_bounds);

    // key_generic_params.extend(value_generic_params);

    let generic_params = key_generic_params;

    let accessors = (0..n)
        .map(|i| i.to_string())
        .map(|i| <Literal as FromStr>::from_str(&i).unwrap());

    quote! {
        impl<#generic_params> Sealed for #tuple_of_key_value_pairs {}

        impl<#generic_params> IntoPairs for #tuple_of_key_value_pairs {
            fn into_pairs(self) -> impl IntoIterator<Item = Result<(CacheKey, CacheValue), Error>> {
                [
                    #(
                        borsh::to_vec(&self.#accessors.1)
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
