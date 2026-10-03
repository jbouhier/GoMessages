//! Settings window content: local HTML + version stamp + live zoom readout
//! + background/tray toggles.

use gomessages::config::VERSION;

use super::prefs::Prefs;

const SETTINGS_TEMPLATE: &str = r#"<!doctype html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
@media (prefers-color-scheme:dark){:root{color-scheme:dark}}
body{font-family:-apple-system,system-ui,sans-serif;margin:0;padding:20px 22px;}
h1{font-size:15px;margin:0 0 4px}
p.sub{font-size:12px;opacity:.6;margin:0 0 16px}
.row{display:flex;align-items:center;gap:8px;margin:10px 0}
.row .label{flex:1;font-size:13px}
.pct{font-size:13px;min-width:44px;text-align:center;font-variant-numeric:tabular-nums}
button{font-size:13px;padding:5px 12px;border-radius:7px;border:1px solid #8884;background:none;cursor:pointer}
button:hover:not(:disabled){background:#8882}
button:disabled{opacity:.35;cursor:default}
button.danger{color:#d33}
[hidden]{display:none!important}
.check{display:flex;align-items:flex-start;gap:8px;margin:10px 0;font-size:13px}
.check input{margin:2px 0 0}
.check.off{opacity:.45}
.hint{display:block;font-size:11px;opacity:.6;margin-top:2px}
hr{border:none;border-top:1px solid #8883;margin:16px 0}
.ver{font-size:11px;opacity:.5;margin-top:14px}
</style></head><body>
<h1>GoMessages Settings</h1>
<p class="sub">Changes apply to the main window immediately.</p>
<div class="row"><span class="label">Page zoom</span>
<button id="zout" onclick="send('zoom-out')">A&minus;</button>
<span class="pct" id="zpct"></span>
<button id="zin" onclick="send('zoom-in')">A+</button>
<button id="zreset" onclick="send('zoom-reset')">Reset</button></div>
<hr>
<label class="check" id="trayrow"><input type="checkbox" id="tray" onchange="pref('tray',this)">
<span><span id="traylabel"></span><span class="hint" id="trayhint"></span></span></label>
<label class="check" id="keeprow"><input type="checkbox" id="keep" onchange="pref('keep_running',this)">
<span>Keep running when the window is closed<span class="hint" id="keephint"></span></span></label>
<label class="check" id="dockrow" hidden><input type="checkbox" id="dock" onchange="pref('hide_dock',this)">
<span>Hide Dock icon<span class="hint">Reopen from the menu bar icon.</span></span></label>
<hr>
<div class="row"><span class="label">Forget this computer (unpair phone on next launch)</span>
<button class="danger" id="unpair" onclick="armUnpair()">Unpair&hellip;</button></div>
<div class="row"><span class="label">Quit GoMessages</span>
<button onclick="send('quit')">Quit</button></div>
<p class="ver">GoMessages __VERSION__</p>
<script>
// Not `ipc`: wry defines window.ipc non-configurable, so a global
// `function ipc` throws at load and kills this whole script.
function send(m){window.ipc.postMessage(m);}
// Rust pushes state here on every zoom change (see app.rs set_zoom).
window.__setZoom=function(pct,idx,last,def){
document.getElementById('zpct').textContent=Math.round(pct)+'%';
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
</body></html>"#;

pub fn settings_html(prefs: &Prefs, tray_ok: bool) -> String {
    SETTINGS_TEMPLATE
        .replace("__VERSION__", VERSION)
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
        for marker in ["__VERSION__", "__ZOOM_STATE__", "__PLATFORM__", "__PREFS__"] {
            assert!(!html.contains(marker), "{marker} left in template");
        }
        assert!(html.contains("__setZoom(25,0,16,7);"));
        assert!(html.contains("\"keep_running\":true"));
    }
}
