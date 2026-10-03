//! Native chat UI (egui/eframe). Window size is remembered automatically
//! by eframe persistence, no manual geometry code needed.
//!
//! Panels are split into methods so the `hybrid` binary (manual winit +
//! embedded webview) can reuse [`ChatApp::show_sidebar`] while the webview
//! owns the conversation area.

use crate::auth::AuthManager;
use crate::backend::Conversation;
use crate::store::ChatStore;
use egui::{Align, CornerRadius, Layout, Margin, RichText, Sense, Stroke};

pub const SIDEBAR_WIDTH: f32 = 300.0;

pub struct ChatApp {
    store: ChatStore,
    auth: AuthManager,
    status: String,
    selected: String,
    search: String,
    draft: String,
    login_error: Option<String>,
}

impl ChatApp {
    pub fn new<B: crate::backend::SyncBackend>(
        storage: Option<&dyn eframe::Storage>,
        backend: B,
    ) -> Self {
        let status = backend.status().to_string();
        let path = ChatStore::default_path().unwrap_or_else(|| "chat.json".into());
        let store = ChatStore::load_or_seed(path, &backend).expect("load chat store");
        let auth_path = AuthManager::default_path().unwrap_or_else(|| "auth.json".into());
        let auth = AuthManager::load(auth_path);
        let selected = storage
            .and_then(|s| s.get_string("selected"))
            .filter(|id| store.data.conversations.iter().any(|c| &c.id == id))
            .or_else(|| store.data.conversations.first().map(|c| c.id.clone()))
            .unwrap_or_default();
        Self {
            store,
            auth,
            status,
            selected,
            search: String::new(),
            draft: String::new(),
            login_error: None,
        }
    }

    pub fn poll_auth(&mut self, ctx: &egui::Context) {
        self.auth.poll(ctx);
    }

    fn visible(&self) -> Vec<Conversation> {
        let q = self.search.to_lowercase();
        self.store
            .data
            .conversations
            .iter()
            .filter(|c| q.is_empty() || c.name.to_lowercase().contains(&q))
            .cloned()
            .collect()
    }

    pub fn show_sidebar(&mut self, ui: &mut egui::Ui) {
        egui::Panel::left("sidebar")
            .resizable(true)
            .default_size(SIDEBAR_WIDTH)
            .show(ui, |ui| {
                ui.add_space(8.0);
                ui.horizontal(|ui| {
                    ui.heading("Messages");
                    ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                        let pending = self.store.pending_count();
                        let label = if pending > 0 {
                            format!("{} pending · {}", pending, self.status)
                        } else {
                            self.status.clone()
                        };
                        ui.label(RichText::new(label).small().weak());
                    });
                });
                ui.add_space(4.0);
                ui.add(
                    egui::TextEdit::singleline(&mut self.search)
                        .hint_text("Search")
                        .desired_width(f32::INFINITY),
                );
                ui.add_space(4.0);
                egui::ScrollArea::vertical().show(ui, |ui| {
                    for convo in self.visible() {
                        let selected = convo.id == self.selected;
                        let fr = egui::Frame::new()
                            .fill(if selected {
                                egui::Color32::from_gray(48)
                            } else {
                                egui::Color32::TRANSPARENT
                            })
                            .corner_radius(CornerRadius::same(8))
                            .inner_margin(Margin::symmetric(8, 6))
                            .show(ui, |ui| {
                                ui.set_width(ui.available_width());
                                ui.horizontal(|ui| {
                                    ui.label(RichText::new(&convo.name).strong());
                                    if convo.unread > 0 {
                                        ui.label(
                                            RichText::new(format!(" {}", convo.unread))
                                                .small()
                                                .color(egui::Color32::WHITE)
                                                .background_color(egui::Color32::from_rgb(
                                                    26, 115, 232,
                                                )),
                                        );
                                    }
                                    ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                                        ui.label(RichText::new(&convo.time).small().weak());
                                    });
                                });
                                ui.label(RichText::new(&convo.snippet).small().weak());
                            });
                        let resp = ui.interact(
                            fr.response.rect,
                            ui.id().with(("convo", convo.id.clone())),
                            Sense::click(),
                        );
                        if resp.clicked() {
                            self.selected = convo.id.clone();
                            let _ = self.store.mark_read(&convo.id);
                        }
                    }
                });
                ui.separator();
                ui.add_space(2.0);
                self.show_account(ui);
                ui.add_space(4.0);
            });
    }

    /// Slim sidebar for the hybrid binary: the web page owns conversations,
    /// so only status + account live here.
    pub fn show_hybrid_sidebar(&mut self, ui: &mut egui::Ui) {
        egui::Panel::left("hybrid-sidebar")
            .resizable(false)
            .default_size(SIDEBAR_WIDTH)
            .show(ui, |ui| {
                ui.add_space(8.0);
                ui.heading("Chat Fast");
                let pending = self.store.pending_count();
                let label = if pending > 0 {
                    format!("{} pending · {}", pending, self.status)
                } else {
                    self.status.clone()
                };
                ui.label(RichText::new(label).small().weak());
                ui.separator();
                ui.add_space(2.0);
                self.show_account(ui);
                ui.add_space(4.0);
            });
    }

    fn show_account(&mut self, ui: &mut egui::Ui) {
        if self.auth.signed_in() {
            let email = self
                .auth
                .tokens
                .as_ref()
                .and_then(|t| t.email.clone())
                .unwrap_or_else(|| "Google account".to_string());
            ui.horizontal(|ui| {
                ui.label(RichText::new(&email).small().strong());
                if ui.small_button("Sign out").clicked() {
                    self.auth.sign_out();
                }
            });
        } else if self.auth.waiting() {
            ui.horizontal(|ui| {
                ui.spinner();
                ui.label(RichText::new("Complete sign-in in your browser…").small());
                if ui.small_button("Cancel").clicked() {
                    self.auth.cancel();
                }
            });
        } else {
            ui.label(RichText::new("Google account").small().weak());
            ui.add(
                egui::TextEdit::singleline(&mut self.auth.client_id)
                    .hint_text("OAuth client ID")
                    .desired_width(f32::INFINITY),
            );
            if ui.button("Sign in with Google").clicked() {
                if let Err(e) = self.auth.begin_sign_in() {
                    self.login_error = Some(format!("{e:#}"));
                } else {
                    self.login_error = None;
                }
            }
            if let Some(e) = &self.login_error {
                ui.label(RichText::new(e).small().color(egui::Color32::LIGHT_RED));
            }
        }
    }

    pub fn show_composer(&mut self, ui: &mut egui::Ui) {
        egui::Panel::bottom("composer").show(ui, |ui| {
            ui.add_space(4.0);
            ui.horizontal(|ui| {
                let resp = ui.add(
                    egui::TextEdit::singleline(&mut self.draft)
                        .hint_text("Text message (queued locally, no backend yet)")
                        .desired_width(ui.available_width() - 70.0),
                );
                let send = ui.button("Send");
                let enter = resp.lost_focus() && ui.input(|i| i.key_pressed(egui::Key::Enter));
                if send.clicked() || enter {
                    if !self.draft.trim().is_empty() {
                        let id = self.selected.clone();
                        let text = std::mem::take(&mut self.draft);
                        let _ = self.store.send(&id, &text);
                    }
                    resp.request_focus();
                }
            });
            ui.add_space(4.0);
        });
    }

    pub fn show_conversation(&mut self, ui: &mut egui::Ui) {
        egui::CentralPanel::default().show(ui, |ui| {
            let title = self
                .store
                .data
                .conversations
                .iter()
                .find(|c| c.id == self.selected)
                .map(|c| c.name.clone());
            let Some(title) = title else {
                ui.centered_and_justified(|ui| ui.label("No conversation"));
                return;
            };
            ui.horizontal(|ui| {
                ui.heading(title);
            });
            ui.separator();

            egui::ScrollArea::vertical()
                .auto_shrink([false; 2])
                .stick_to_bottom(true)
                .show(ui, |ui| {
                    for msg in self.store.messages(&self.selected.clone()) {
                        ui.with_layout(
                            if msg.from_me {
                                Layout::right_to_left(Align::TOP)
                            } else {
                                Layout::left_to_right(Align::TOP)
                            },
                            |ui| {
                                let fill = if msg.from_me {
                                    egui::Color32::from_rgb(26, 115, 232)
                                } else {
                                    egui::Color32::from_gray(58)
                                };
                                egui::Frame::new()
                                    .fill(fill)
                                    .corner_radius(CornerRadius::same(14))
                                    .inner_margin(Margin::symmetric(12, 8))
                                    .stroke(Stroke::NONE)
                                    .show(ui, |ui| {
                                        ui.set_max_width(420.0);
                                        ui.label(
                                            RichText::new(&msg.text).color(egui::Color32::WHITE),
                                        );
                                        let meta = if msg.pending {
                                            format!("{} · sending…", msg.time)
                                        } else {
                                            msg.time.clone()
                                        };
                                        ui.label(
                                            RichText::new(meta)
                                                .size(10.0)
                                                .color(egui::Color32::from_white_alpha(170)),
                                        );
                                    });
                            },
                        );
                        ui.add_space(6.0);
                    }
                });
        });
    }
}

impl eframe::App for ChatApp {
    fn save(&mut self, storage: &mut dyn eframe::Storage) {
        storage.set_string("selected", self.selected.clone());
    }

    fn ui(&mut self, ui: &mut egui::Ui, _frame: &mut eframe::Frame) {
        let ctx = ui.ctx().clone();
        self.poll_auth(&ctx);
        self.show_sidebar(ui);
        // Composer before central so central takes leftovers.
        self.show_composer(ui);
        self.show_conversation(ui);
    }
}
