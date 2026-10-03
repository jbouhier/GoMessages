pub mod config;
pub mod paths;

// Native prototype track (egui UI + OAuth + store). Behind the `native`
// feature so default builds stay lean; enable with `--features native`.
#[cfg(feature = "native")]
pub mod auth;
#[cfg(feature = "native")]
pub mod backend;
#[cfg(feature = "native")]
pub mod store;
#[cfg(feature = "native")]
pub mod ui;
