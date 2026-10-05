//! Settings window content: local HTML + live zoom readout
//! + background/tray toggles.

use super::prefs::Prefs;

const SETTINGS_TEMPLATE: &str = r##"<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root{color-scheme:light;--bg:#f5f7fb;--surface:#fff;--text:#202633;--muted:#697386;--line:#e4e9f0;--accent:#2b6de9;--accent-soft:#e9f0ff;--control:#f8fafc;--danger:#b42334;--danger-soft:#fff0f1;--shadow:0 8px 26px #253b6410}
@media(prefers-color-scheme:dark){:root{color-scheme:dark;--bg:#171b22;--surface:#222832;--text:#f3f6fb;--muted:#a2adbe;--line:#38414e;--accent:#86afff;--accent-soft:#28426d;--control:#2b323e;--danger:#ff9ca6;--danger-soft:#493038;--shadow:0 8px 26px #0002}}
*{box-sizing:border-box}
html,body{min-height:100%}
body{margin:0;background:var(--bg);color:var(--text);font:13px/1.4 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
.page{max-width:560px;margin:auto;padding:24px 28px 20px}
.hero{display:flex;align-items:center;gap:13px;margin-bottom:23px}
.mark{display:grid;place-items:center;width:44px;height:44px;flex:none;border-radius:13px;background:var(--accent-soft)}
.mark svg{width:28px;height:28px}
h1{font-size:23px;line-height:44px;letter-spacing:-.035em;margin:0;font-weight:700}
section{margin:0 0 20px}
h2{font-size:10px;font-weight:750;letter-spacing:.12em;text-transform:uppercase;color:var(--muted);margin:0 0 8px 3px}
.card{background:var(--surface);border:1px solid var(--line);border-radius:15px;box-shadow:var(--shadow);overflow:hidden}
.item{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:65px;padding:12px 15px}
.item+.item{border-top:1px solid var(--line)}
.copy{min-width:0;flex:1}
.title{display:block;font-weight:620;font-size:13px;line-height:1.3}
.hint{display:block;color:var(--muted);font-size:11px;line-height:1.35;margin-top:4px}
button,select{font:inherit}
button{cursor:pointer}
button:disabled{opacity:.38;cursor:default}
button:focus-visible,select:focus-visible,input:focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.zoom{display:flex;align-items:center;flex:none;border:1px solid var(--line);border-radius:10px;background:var(--control);overflow:hidden}
.zoom button{border:0;background:transparent;color:var(--text);width:29px;height:31px;padding:0;font-size:17px;line-height:1}
.zoom button:hover:not(:disabled){background:var(--accent-soft)}
.zoom .reset{font-size:16px;border-left:1px solid var(--line);color:var(--accent)}
.pct{min-width:43px;text-align:center;font-variant-numeric:tabular-nums;font-size:12px;font-weight:650}
.sound-controls{display:flex;align-items:center;gap:8px;flex:none}
.preview{display:grid;place-items:center;width:33px;height:33px;flex:none;border:1px solid var(--line);border-radius:9px;background:var(--control);color:var(--accent)}
.preview:hover:not(:disabled){background:var(--accent-soft)}
.preview svg{width:13px;height:13px;fill:currentColor;margin-left:2px}
.switch{appearance:none;-webkit-appearance:none;flex:none;width:36px;height:22px;border-radius:20px;border:1px solid var(--line);background:#aeb8c5;position:relative;margin:0;cursor:pointer;transition:background .15s}
.switch:before{content:"";position:absolute;top:2px;left:2px;width:16px;height:16px;border-radius:50%;background:white;box-shadow:0 1px 3px #0003;transition:transform .15s}
.switch:checked{background:var(--accent);border-color:var(--accent)}
.switch:checked:before{transform:translateX(14px)}
.switch:disabled{cursor:default}
.item.off{opacity:.48}
[hidden]{display:none!important}
.action{flex:none;border:1px solid var(--line);border-radius:8px;padding:6px 11px;background:var(--control);color:var(--text);font-size:12px;font-weight:600}
.action:hover{background:var(--accent-soft)}
.action.danger{border-color:transparent;background:var(--danger-soft);color:var(--danger)}
.action.danger:hover{filter:brightness(.96)}
</style></head><body><div class="page">
<header class="hero"><span class="mark" aria-hidden="true"><svg viewBox="0 0 64 64"><rect x="4" y="9" width="56" height="37" rx="18" fill="#8AB4F8"/><polygon points="13,42 9,56 22,45" fill="#8AB4F8"/><rect x="11.5" y="18" width="41" height="21" rx="10" fill="#1A73E8"/></svg></span><h1>Settings</h1></header>
<main>
<section><h2>Display &amp; alerts</h2><div class="card">
<div class="item"><div class="copy"><span class="title">Page zoom</span><span class="hint">Saved for this monitor.</span></div><div class="zoom">
<button id="zout" aria-label="Zoom out" title="Zoom out" onclick="send('zoom-out')">&minus;</button>
<span class="pct" id="zpct" aria-live="polite"></span>
<button id="zin" aria-label="Zoom in" title="Zoom in" onclick="send('zoom-in')">+</button>
<button class="reset" id="zreset" aria-label="Reset zoom" title="Reset zoom" onclick="send('zoom-reset')">↺</button>
</div></div>
<div class="item"><div class="copy"><label class="title" for="sound">Notification sound</label><span class="hint">__SOUND_HINT__</span></div>
<div class="sound-controls"><input class="switch" id="sound" type="checkbox" aria-label="Play notification sounds" onchange="send('sound:'+(this.checked?'on':'off'))"><button class="preview" id="soundpreview" type="button" aria-label="Play notification sound" title="Play notification sound" onclick="send('sound-preview')"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 1.8v12.4c0 .6.7 1 1.2.7l9.4-6.2a.8.8 0 0 0 0-1.4L4.2 1.1C3.7.8 3 1.2 3 1.8Z"/></svg></button></div></div>
</div></section>
<section><h2>App behavior</h2><div class="card">
<label class="item" id="trayrow"><span class="copy"><span class="title" id="traylabel"></span><span class="hint" id="trayhint"></span></span><input class="switch" type="checkbox" id="tray" onchange="pref('tray',this)"></label>
<label class="item" id="keeprow"><span class="copy"><span class="title">Keep running when closed</span><span class="hint" id="keephint"></span></span><input class="switch" type="checkbox" id="keep" onchange="pref('keep_running',this)"></label>
<label class="item" id="dockrow" hidden><span class="copy"><span class="title">Hide Dock icon</span><span class="hint">Reopen from the menu bar icon.</span></span><input class="switch" type="checkbox" id="dock" onchange="pref('hide_dock',this)"></label>
</div></section>
<section><h2>Account</h2><div class="card"><div class="item"><div class="copy"><span class="title">Forget this computer</span><span class="hint">Unpair your phone on next launch.</span></div><button class="action danger" id="unpair" onclick="armUnpair()">Unpair&hellip;</button></div></div></section>
</main>
</div>
<script>
// Not `ipc`: wry defines window.ipc non-configurable, so a global
// `function ipc` throws at load and kills this whole script.
function send(m){window.ipc.postMessage(m);}
// Rust pushes state here on every zoom change (see app.rs set_zoom).
window.__setZoom=function(pct,idx,last,def){
document.getElementById('zpct').textContent=pct+'%';
document.getElementById('zout').disabled=idx<=0;
document.getElementById('zin').disabled=idx>=last;
document.getElementById('zreset').disabled=idx===def;};
__setZoom(__ZOOM_STATE__);
// Win/Linux have no menubar; macOS menu items catch Cmd+=/-/0 first.
window.addEventListener('keydown',function(e){
if(!(e.metaKey||e.ctrlKey))return;var k=e.key,c=e.code,m=null;
if(k==='='||k==='+'||c==='Equal'||c==='NumpadAdd')m='zoom-in';
else if(k==='-'||k==='_'||c==='Minus'||c==='NumpadSubtract')m='zoom-out';
else if(k==='0'||c==='Digit0'||c==='Numpad0')m='zoom-reset';
if(m){send(m);e.preventDefault();}},true);
// Background / tray. Rust validates and pushes the result back via __setPrefs.
var OS='__PLATFORM__';
function pref(k,el){send('pref:'+k+':'+(el.checked?1:0));}
function en(id,on){document.getElementById(id).disabled=!on;document.getElementById(id+'row').classList.toggle('off',!on);}
window.__setPrefs=function(p){
var mac=OS==='macos';
document.getElementById('traylabel').textContent=mac?'Show menu bar icon':'Show tray icon';
document.getElementById('trayhint').textContent=!p.tray_ok?'No tray available on this desktop.':(OS==='linux'?'Needs AppIndicator support (GNOME: AppIndicator extension).':'');
document.getElementById('tray').checked=p.tray;
document.getElementById('sound').checked=p.notification_sound!=='off';
document.getElementById('keep').checked=p.keep_running&&(mac||p.tray);
document.getElementById('keephint').textContent=mac?'Reopen from the Dock'+(p.tray?' or menu bar icon.':'.'):(p.tray?'Reopen from the tray icon.':'Needs the tray icon.');
en('keep',mac||p.tray);
document.getElementById('dockrow').hidden=!mac;
document.getElementById('dock').checked=p.hide_dock;
en('dock',p.tray);};
__setPrefs(__PREFS__);
// Two-click confirm: wry has no WKUIDelegate, so confirm() is a silent false on macOS.
var armed=null;
function armUnpair(){var b=document.getElementById('unpair');
if(armed){clearTimeout(armed);send('unpair');return;}
b.textContent='Click again to unpair';
armed=setTimeout(function(){armed=null;b.innerHTML='Unpair&hellip;';},4000);}
</script>
</body></html>"##;

pub fn settings_html(prefs: &Prefs, tray_ok: bool) -> String {
    SETTINGS_TEMPLATE
        .replace(
            "__SOUND_HINT__",
            if cfg!(target_os = "macos") {
                "macOS notification tone for new messages."
            } else {
                "System alert for new messages."
            },
        )
        .replace("__ZOOM_STATE__", &zoom_state_args(prefs.zoom_idx))
        .replace("__PLATFORM__", PLATFORM)
        .replace("__PREFS__", &prefs_state_json(prefs, tray_ok))
}

const PLATFORM: &str = if cfg!(target_os = "macos") {
    "macos"
} else if cfg!(target_os = "windows") {
    "windows"
} else {
    "linux"
};

/// JSON arg for `__setPrefs(p)`.
pub fn prefs_state_json(prefs: &Prefs, tray_ok: bool) -> String {
    serde_json::json!({
        "keep_running": prefs.keep_running,
        "tray": prefs.tray,
        "hide_dock": prefs.hide_dock,
        "notification_sound": prefs.notification_sound,
        "tray_ok": tray_ok,
    })
    .to_string()
}

/// JS args for `__setZoom(pct, idx, last, def)`.
pub fn zoom_state_args(idx: usize) -> String {
    use super::webview::{DEFAULT_IDX, PRESETS};
    format!(
        "{},{},{},{}",
        PRESETS[idx],
        idx,
        PRESETS.len() - 1,
        DEFAULT_IDX
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn template_fully_filled() {
        let prefs = Prefs {
            zoom_idx: 0,
            ..Prefs::default()
        };
        let html = settings_html(&prefs, true);
        for marker in [
            "__ZOOM_STATE__",
            "__PLATFORM__",
            "__PREFS__",
            "__SOUND_HINT__",
        ] {
            assert!(!html.contains(marker), "{marker} left in template");
        }
        assert!(html.contains("__setZoom(25,0,16,7);"));
        assert!(html.contains("\"keep_running\":true"));
        assert!(html.contains("onclick=\"send('sound-preview')\""));
        assert!(!html.contains("Soft bell"));
        assert!(!html.contains("Preferences for this device"));
        assert!(!html.contains("<footer"));
    }
}
