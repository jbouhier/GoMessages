//! Local store: conversations + messages + outbox, persisted as JSON.
//!
//! The store is backend-agnostic. Today it seeds from `MockBackend` on first
//! run; a real `SyncBackend` would merge server deltas here and drain the
//! outbox (pending sends) instead of leaving them pending.

use crate::backend::{Conversation, Message};
use std::collections::HashMap;
use std::path::PathBuf;

#[derive(Debug, Default, serde::Serialize, serde::Deserialize)]
pub struct StoreData {
    pub conversations: Vec<Conversation>,
    pub messages: HashMap<String, Vec<Message>>,
    pub next_id: u64,
}

pub struct ChatStore {
    path: PathBuf,
    pub data: StoreData,
}

impl ChatStore {
    pub fn default_path() -> Option<PathBuf> {
        crate::paths::data_dir().map(|p| p.join(crate::config::CHAT_FILE))
    }

    /// Load from disk, or seed from the backend and persist. A corrupt file
    /// is quarantined aside (never a startup crash) and reseeded.
    pub fn load_or_seed<B: crate::backend::SyncBackend>(
        path: PathBuf,
        backend: &B,
    ) -> anyhow::Result<Self> {
        if path.exists() {
            match std::fs::read(&path)
                .map_err(anyhow::Error::from)
                .and_then(|b| serde_json::from_slice::<StoreData>(&b).map_err(anyhow::Error::from))
            {
                Ok(data) => return Ok(Self { path, data }),
                Err(e) => {
                    eprintln!("store corrupt ({e:#}), reseeding; backup kept");
                    let bak = path.with_extension("json.corrupt");
                    let _ = std::fs::rename(&path, bak);
                }
            }
        }
        let conversations = backend.conversations();
        let mut messages = HashMap::new();
        let mut max_id = 0u64;
        for c in &conversations {
            let msgs = backend.messages(&c.id);
            max_id = max_id.max(msgs.iter().map(|m| m.id).max().unwrap_or(0));
            messages.insert(c.id.clone(), msgs);
        }
        let data = StoreData {
            conversations,
            messages,
            next_id: max_id + 1,
        };
        let store = Self { path, data };
        store.save()?;
        Ok(store)
    }

    pub fn save(&self) -> anyhow::Result<()> {
        if let Some(parent) = self.path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        let bytes = serde_json::to_vec_pretty(&self.data)?;
        std::fs::write(&self.path, bytes)?;
        Ok(())
    }

    /// Queue an outgoing message. Marked pending (no backend to drain it yet).
    pub fn send(&mut self, conversation_id: &str, text: &str) -> anyhow::Result<()> {
        let text = text.trim();
        if text.is_empty() {
            return Ok(());
        }
        let id = self.data.next_id;
        self.data.next_id += 1;
        let msg = Message {
            id,
            from_me: true,
            text: text.to_string(),
            time: now_hhmm(),
            pending: true,
        };
        self.data
            .messages
            .entry(conversation_id.to_string())
            .or_default()
            .push(msg);
        if let Some(c) = self
            .data
            .conversations
            .iter_mut()
            .find(|c| c.id == conversation_id)
        {
            c.snippet = text.to_string();
            c.time = now_hhmm();
        }
        self.save()
    }

    pub fn mark_read(&mut self, conversation_id: &str) -> anyhow::Result<()> {
        if let Some(c) = self
            .data
            .conversations
            .iter_mut()
            .find(|c| c.id == conversation_id)
        {
            if c.unread > 0 {
                c.unread = 0;
                self.save()?;
            }
        }
        Ok(())
    }

    pub fn messages(&self, conversation_id: &str) -> &[Message] {
        self.data
            .messages
            .get(conversation_id)
            .map(Vec::as_slice)
            .unwrap_or(&[])
    }

    pub fn pending_count(&self) -> usize {
        self.data
            .messages
            .values()
            .flat_map(|v| v.iter())
            .filter(|m| m.pending)
            .count()
    }
}

fn now_hhmm() -> String {
    chrono::Local::now().format("%H:%M").to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::backend::MockBackend;

    fn tmp_path(name: &str) -> PathBuf {
        std::env::temp_dir().join(format!("gomsg-test-{}-{}", std::process::id(), name))
    }

    #[test]
    fn seeds_from_backend_and_roundtrips() {
        let path = tmp_path("seed.json");
        let _ = std::fs::remove_file(&path);
        let b = MockBackend;
        let store = ChatStore::load_or_seed(path.clone(), &b).unwrap();
        assert_eq!(store.data.conversations.len(), 3);
        assert!(!store.messages("c1").is_empty());
        let reloaded = ChatStore::load_or_seed(path.clone(), &b).unwrap();
        assert_eq!(reloaded.data.next_id, store.data.next_id);
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn send_appends_pending_and_updates_snippet() {
        let path = tmp_path("send.json");
        let _ = std::fs::remove_file(&path);
        let b = MockBackend;
        let mut store = ChatStore::load_or_seed(path.clone(), &b).unwrap();
        let before = store.messages("c1").len();
        store.send("c1", "hello from test").unwrap();
        let msgs = store.messages("c1");
        assert_eq!(msgs.len(), before + 1);
        let last = msgs.last().unwrap();
        assert!(last.from_me && last.pending);
        assert_eq!(last.text, "hello from test");
        assert_eq!(store.pending_count(), 1);
        let convo = store
            .data
            .conversations
            .iter()
            .find(|c| c.id == "c1")
            .unwrap();
        assert_eq!(convo.snippet, "hello from test");
        let _ = std::fs::remove_file(&path);
    }

    #[test]
    fn corrupt_file_reseeds_instead_of_crashing() {
        let path = tmp_path("corrupt.json");
        let _ = std::fs::remove_file(&path);
        std::fs::write(&path, b"{not json").unwrap();
        let b = MockBackend;
        let store = ChatStore::load_or_seed(path.clone(), &b).unwrap();
        assert_eq!(store.data.conversations.len(), 3);
        assert!(path.with_extension("json.corrupt").exists());
        let _ = std::fs::remove_file(&path);
        let _ = std::fs::remove_file(path.with_extension("json.corrupt"));
    }

    #[test]
    fn mark_read_zeroes_unread() {
        let path = tmp_path("read.json");
        let _ = std::fs::remove_file(&path);
        let b = MockBackend;
        let mut store = ChatStore::load_or_seed(path.clone(), &b).unwrap();
        assert!(store.data.conversations.iter().any(|c| c.unread > 0));
        store.mark_read("c1").unwrap();
        assert_eq!(
            store
                .data
                .conversations
                .iter()
                .find(|c| c.id == "c1")
                .unwrap()
                .unread,
            0
        );
        let _ = std::fs::remove_file(&path);
    }
}
