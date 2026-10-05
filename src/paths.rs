//! App data dir. One triple, no migration history.

use std::path::PathBuf;

use crate::config;

/// Resolve the data dir, creating nothing. Callers create what they need.
pub fn data_dir() -> Option<PathBuf> {
    if std::env::var_os("GOMESSAGES_MARKETING_DEMO").as_deref() == Some(std::ffi::OsStr::new("1")) {
        return std::env::var_os("GOMESSAGES_MARKETING_DATA_DIR").map(PathBuf::from);
    }
    directories::ProjectDirs::from(config::DATA_QUALIFIER, config::DATA_ORG, config::DATA_APP)
        .map(|p| p.data_dir().to_path_buf())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn data_dir_uses_go_messages_triple() {
        let dir = data_dir().expect("data dir");
        let name = dir.file_name().and_then(|n| n.to_str()).unwrap_or_default();
        // macOS: dev.go-messages.go-messages, Linux: go-messages.
        assert!(
            name.contains("go-messages"),
            "unexpected data dir name: {name}"
        );
    }
}
