extern crate proc_macro;

use core::panic;

use proc_macro::TokenStream;
use quote::{quote, ToTokens};
use syn::{parse_macro_input, parse_str, FnArg, Ident, ItemFn, Pat, PatType, Type};

/// An attribute procedural macro that can be used to expose a function to the host.
/// It does so by wrapping the [item] tokenstream in a new function that can be called by the host.
/// The wrapper function will have the same name as the original function, but with "_guest" appended to it.
/// The wrapper functions parameters will be converted to WASM supported types. When called, the wrapper function
/// calls the original function by converting the parameters back to their intended types using .into().
#[proc_macro_attribute]
pub fn expose(_: TokenStream, item: TokenStream) -> TokenStream {
    let input = parse_macro_input!(item as ItemFn);
    let name = &input.sig.ident;
    let input_args = &input.sig.inputs;
    let new_name = Ident::new(&format!("{}_guest", name), name.span()); // Create a new name for the generated function(name that will be called by the host)

    let full_params = input_args.iter().map(|fn_arg| {
        // A typed argument is a parameter. An untyped(reciever) argument is a self parameter.
        if let FnArg::Typed(PatType { pat, ty, .. }) = fn_arg {
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
            // Explicitly note this will panic on _ parameters.
            if let Pat::Wild(_) = **pat {
                panic!("Unused variables not supported.");
            }
        }
        panic!("Unsupported function parameter format.");
    });

    // Collect all parameter names and types into separate vectors.
    let param_names: Vec<_> = full_params.clone().map(|(name, _)| name).collect();
    // Copy the iterator so we can use it again.
    let param_names_cloned: Vec<_> = param_names.clone();
    let param_types: Vec<_> = full_params.map(|(_, ty)| ty).collect();

    // Extract the original function's return type. This must be a WASM supported type.
    let return_type = &input.sig.output;
    let output = quote! {
        // Need to include the original function in the output, so contract can call itself
        #input
        #[no_mangle]
        pub extern "C" fn #new_name(#(#param_names: #param_types), *) #return_type {
            // .into() uses the From() on each argument in the iterator to convert it to the type we want. 70% sure about this statement.
            #name(#(#param_names_cloned.into()),*) // This means that every parameter type must implement From<i64>(except for the supported primitive types).
        }
    };
    TokenStream::from(output)
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
