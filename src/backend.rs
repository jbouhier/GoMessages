//! Sync seam + mock data.
//!
//! A shippable client must implement `SyncBackend` against the (undocumented)
//! Messages-for-web protocol:
//!   1. Google OAuth sign-in for the web client scope.
//!   2. Device pairing: account link + emoji confirm + Bluetooth proximity
//!      check (this is what kills plain webviews).
//!   3. Relay WebSocket to the phone (conversations, deltas, receipts).
//!   4. Send path (SMS/RCS via phone), media upload, retries.
//!
//! None of that exists yet. Anything marked mock is placeholder.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Conversation {
    pub id: String,
    pub name: String,
    pub snippet: String,
    pub time: String,
    pub unread: usize,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    pub id: u64,
    pub from_me: bool,
    pub text: String,
    pub time: String,
    /// True while queued locally without a backend to deliver it.
    #[serde(default)]
    pub pending: bool,
}

pub trait SyncBackend {
    fn conversations(&self) -> Vec<Conversation>;
    fn messages(&self, conversation_id: &str) -> Vec<Message>;
    fn status(&self) -> &'static str;
}

/// Placeholder backend: fixed sample data, no network.
pub struct MockBackend;

impl SyncBackend for MockBackend {
    fn status(&self) -> &'static str {
        "offline, sync not implemented"
    }

    fn conversations(&self) -> Vec<Conversation> {
        vec![
            Conversation {
                id: "c1".into(),
                name: "Ada Lovelace".into(),
                snippet: "The prototype renders at 120fps now".into(),
                time: "09:41".into(),
                unread: 2,
            },
            Conversation {
                id: "c2".into(),
                name: "Grace Hopper".into(),
                snippet: "Debugging is twice as hard…".into(),
                time: "Yesterday".into(),
                unread: 0,
            },
            Conversation {
                id: "c3".into(),
                name: "+1 555 013 3456".into(),
                snippet: "Your verification code is 493201".into(),
                time: "Mon".into(),
                unread: 0,
            },
        ]
    }

    fn messages(&self, conversation_id: &str) -> Vec<Message> {
        match conversation_id {
            "c1" => vec![
                Message {
                    id: 1,
                    from_me: false,
                    text: "Did the new shell pass login?".into(),
                    time: "09:37".into(),
                    pending: false,
                },
                Message {
                    id: 2,
                    from_me: true,
                    text: "WebView has no Bluetooth, pairing fails.".into(),
                    time: "09:39".into(),
                    pending: false,
                },
                Message {
                    id: 3,
                    from_me: false,
                    text: "The prototype renders at 120fps now".into(),
                    time: "09:41".into(),
                    pending: false,
                },
            ],
            "c2" => vec![Message {
                id: 1,
                from_me: false,
                text: "Debugging is twice as hard as writing the code in the first place.".into(),
                time: "Yesterday".into(),
                pending: false,
            }],
            _ => vec![Message {
                id: 1,
                from_me: false,
                text: "Your verification code is 493201".into(),
                time: "Mon".into(),
                pending: false,
            }],
        }
    }
}
