extern crate proc_macro;

use core::panic;

use proc_macro::TokenStream;
use proc_macro2::Span;
use quote::{quote, ToTokens};
use syn::{
    parse_macro_input, parse_str, Fields, FnArg, Ident, ItemEnum, ItemFn, Pat, PatType, Type,
};

/// An attribute procedural macro that makes a function visible to the VM host.
/// It does so by wrapping the `item` tokenstream in a new function that can be called by the host.
/// The wrapper function will have the same name as the original function, but with "_guest" appended to it.
/// The wrapper functions parameters will be converted to WASM supported types. When called, the wrapper function
/// calls the original function by converting the parameters back to their intended types using .into().
#[proc_macro_attribute]
pub fn public(_: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as ItemFn);
    let name = &input.sig.ident;
    let input_args = &input.sig.inputs;
    let new_name = Ident::new(&format!("{}_guest", name), name.span()); // Create a new name for the generated function(name that will be called by the host)
    let empty_param = Ident::new("ctx", Span::call_site()); // Create an empty parameter for the generated function
    let full_params = input_args.iter().enumerate().map(|(index, fn_arg)| {
        // A typed argument is a parameter. An untyped (receiver) argument is a `self` parameter.
        if let FnArg::Typed(PatType { pat, ty, .. }) = fn_arg {
            // ensure first parameter is Context
            if index == 0 && !is_context(ty) {
                panic!("First parameter must be Context.");
            }

            if let Pat::Ident(ref pat_ident) = **pat {
                let param_name = &pat_ident.ident;
                // We only set the type to i64 if it is not a supported WASM primitive.
                let param_type = if is_supported_primitive(ty) {
                    ty.to_token_stream()
                } else {
                    parse_str::<Type>("i64")
                        .expect("valid i64 type")
                        .to_token_stream()
                };
                return (param_name, param_type);
            }
            // add unused variable
            if let Pat::Wild(_) = **pat {
                if is_context(ty) {
                    return (
                        &empty_param,
                        parse_str::<Type>("i64")
                            .expect("valid i64 type")
                            .to_token_stream(),
                    );
                } else {
                    panic!("Unused variables only supported for Program.")
                }
            }
        }
        panic!("Unsupported function parameter format.");
    });

    // Collect all parameter names and types into separate vectors.
    let (param_names, param_types): (Vec<_>, Vec<_>) = full_params.unzip();

    // Extract the original function's return type. This must be a WASM supported type.
    let return_type = &input.sig.output;
    let output = quote! {
        // Need to include the original function in the output, so contract can call itself
        #input
        #[no_mangle]
        pub extern "C" fn #new_name(#(#param_names: #param_types), *) #return_type {
            // .into() uses the From() on each argument in the iterator to convert it to the type we want. 70% sure about this statement.
            #name(#(#param_names.into()),*) // This means that every parameter type must implement From<i64>(except for the supported primitive types).
        }
    };

    TokenStream::from(output)
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
         #[derive(Clone, Copy, Debug)]
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

/// Returns whether the type_path represents a supported primitive type.
fn is_supported_primitive(type_path: &std::boxed::Box<Type>) -> bool {
    if let Type::Path(ref type_path) = **type_path {
        let ident = &type_path.path.segments[0].ident;
        let ident_str = ident.to_string();
        matches!(ident_str.as_str(), "i32" | "i64" | "bool")
    } else {
        false
    }
}

/// Returns whether the type_path represents a Context type.
fn is_context(type_path: &std::boxed::Box<Type>) -> bool {
    if let Type::Path(ref type_path) = **type_path {
        let ident = &type_path.path.segments[0].ident;
        let ident_str = ident.to_string();
        ident_str == "Program"
    } else {
        false
    }
}
