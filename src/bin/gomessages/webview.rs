//! Embedded page: keybind/unread/notification init script + webview bounds math.

use gomessages::config::TOP_INSET;
use winit::window::Window;

/// Chromium-style zoom presets, percent. Rust owns the index so settings
/// buttons and shortcuts stay relative across restarts.
pub const PRESETS: [f64; 17] = [
    25.0, 33.3, 50.0, 66.7, 75.0, 80.0, 90.0, 100.0, 110.0, 125.0, 150.0, 175.0, 200.0, 250.0,
    300.0, 400.0, 500.0,
];
pub const DEFAULT_IDX: usize = 7;

/// Hosts the page is allowed to navigate to. Everything else opens in the
/// system browser, so a clicked link can never silently replace the app
/// (and the app never renders attacker-controlled origins as itself).
const ALLOWED_HOSTS: &[&str] = &["messages.google.com", "accounts.google.com"];

/// Pure policy, unit-tested below. Unknown schemes, unparsable URLs, and
/// lookalike domains (`messages.google.com.evil.com`) are denied.
pub fn navigation_allowed(url: &str) -> bool {
    if url.is_empty() || url == "about:blank" {
        return true;
    }
    let Ok(parsed) = url::Url::parse(url) else {
        return false;
    };
    if !matches!(parsed.scheme(), "http" | "https") {
        return false;
    }
    let host = parsed.host_str().unwrap_or_default().to_lowercase();
    ALLOWED_HOSTS
        .iter()
        .any(|h| host == *h || host.ends_with(&format!(".{h}")))
}

/// Runs before page scripts on every document load. Rust applies zoom through
/// the webview, keeping page popovers and fixed overlays in viewport coordinates.
/// Binds Ctrl+=/-/0 for Windows/Linux
/// (no menubar there). On macOS the View menu owns Cmd+=/-/0: child webviews
/// never see Cmd+key events.
pub const INIT_JS: &str = r##"(function(){
var splash=document.createElement('div');
splash.setAttribute('style','position:fixed;inset:0;z-index:2147483647;background:#ffffff;display:flex;align-items:center;justify-content:center;transition:opacity .3s;');
try{if(window.matchMedia&&matchMedia('(prefers-color-scheme: dark)').matches){splash.style.background='#1f1f1f';}}catch(e){}
splash.innerHTML='<svg style="width:min(128px,50vw,50vh);height:min(128px,50vw,50vh)" viewBox="0 0 64 64"><rect x="4" y="9" width="56" height="37" rx="18" fill="#8AB4F8"/><polygon points="13,42 9,56 22,45" fill="#8AB4F8"/><rect x="11.5" y="18" width="41" height="21" rx="10" fill="#1A73E8"/></svg>';
function showsplash(){var t=document.documentElement||document.body;if(t){t.appendChild(splash);return true;}return false;}
function hidesplash(){splash.style.opacity='0';setTimeout(function(){splash.remove();},350);}
if(!showsplash()){document.addEventListener('DOMContentLoaded',showsplash);}
window.addEventListener('load',function(){setTimeout(hidesplash,400);});
setTimeout(hidesplash,15000);
var zoomToastTimer;
window.__gomsgShowZoom=function(pct){
var scale=pct/100;
var toast=document.getElementById('gomsg-zoom-toast');
if(!toast){
var root=document.documentElement||document.body;if(!root)return;
toast=document.createElement('div');toast.id='gomsg-zoom-toast';
toast.setAttribute('role','status');toast.setAttribute('aria-live','polite');
toast.style.cssText='all:initial;position:fixed;z-index:2147483646;pointer-events:none;box-sizing:border-box;padding:9px 13px;border-radius:10px;background:rgba(30,35,44,.88);color:#fff;font:600 13px/1 -apple-system,BlinkMacSystemFont,sans-serif;white-space:nowrap;box-shadow:0 4px 16px rgba(0,0,0,.18);opacity:0;transition:opacity .25s ease;transform-origin:top right;';
root.appendChild(toast);
}
toast.textContent=pct+'%';
toast.style.top=16/scale+'px';toast.style.right=16/scale+'px';
toast.style.transform='scale('+1/scale+')';
clearTimeout(zoomToastTimer);
toast.getBoundingClientRect();toast.style.opacity='1';
zoomToastTimer=setTimeout(function(){toast.style.opacity='0';},1500);
};
window.addEventListener('keydown',function(e){
if(!(e.metaKey||e.ctrlKey))return;
var k=e.key||'';
function isIn(list){for(var i=0;i<list.length;i++){if(k===list[i])return true;}return false;}
var plus=isIn(['=','+'])||e.code==='Equal'||e.code==='NumpadAdd';
var minus=isIn(['-','_'])||e.code==='Minus'||e.code==='NumpadSubtract';
var zero=(k==='0')||e.code==='Digit0'||e.code==='Numpad0';
// Ctrl+, opens Settings on Windows/Linux (no menubar there; macOS menu wins).
if(k===','||e.code==='Comma'){try{window.ipc.postMessage('open-settings');}catch(x){}e.preventDefault();e.stopImmediatePropagation();return;}
var action=plus?'zoom-in':minus?'zoom-out':zero?'zoom-reset':null;
if(action){try{window.ipc.postMessage(action);}catch(x){}e.preventDefault();e.stopImmediatePropagation();}
},true);
// Web notifications. WKWebView has no Notification API, so the page's own
// "new message" alerts went nowhere. This stand-in reports permission as
// granted and forwards each notification to Rust (banner + sound there).
// The page still decides when to notify (background only), and clicking the
// banner runs the page's own onclick, which opens that conversation.
if(location.hostname==='messages.google.com'){(function(){
var seq=0,live=Object.create(null),order=[];
function post(n){try{window.ipc.postMessage('notify:'+JSON.stringify(n));}catch(e){}}
function keep(id,n){live[id]=n;order.push(id);if(order.length>50){delete live[order.shift()];}}
function GoNotification(title,opts){
opts=opts||{};var id=String(++seq);
this.title=String(title==null?'':title);this.body=String(opts.body||'');this.tag=String(opts.tag||'');
this.data=opts.data===undefined?null:opts.data;this.icon=opts.icon||'';this.silent=!!opts.silent;
this.onclick=null;this.onclose=null;this.onshow=null;this.onerror=null;this.__id=id;this.__l={};
keep(id,this);post({id:id,title:this.title,body:this.body,tag:this.tag});
var self=this;setTimeout(function(){self.__fire('show');},0);}
GoNotification.prototype.addEventListener=function(t,f){(this.__l[t]=this.__l[t]||[]).push(f);};
GoNotification.prototype.removeEventListener=function(t,f){var a=this.__l[t]||[];var i=a.indexOf(f);if(i>=0)a.splice(i,1);};
GoNotification.prototype.__fire=function(t){var ev={type:t,target:this,currentTarget:this,defaultPrevented:false,preventDefault:function(){this.defaultPrevented=true;}};
var h=this['on'+t];if(typeof h==='function'){try{h.call(this,ev);}catch(e){}}
var a=(this.__l[t]||[]).slice();for(var i=0;i<a.length;i++){try{a[i].call(this,ev);}catch(e){}}};
GoNotification.prototype.close=function(){if(live[this.__id]){delete live[this.__id];this.__fire('close');}};
GoNotification.permission='granted';GoNotification.maxActions=0;
GoNotification.requestPermission=function(cb){if(typeof cb==='function'){try{cb('granted');}catch(e){}}return Promise.resolve('granted');};
try{Object.defineProperty(window,'Notification',{value:GoNotification,writable:true,configurable:true});}catch(e){window.Notification=GoNotification;}
window.__gomsgNotifClick=function(id){var n=live[String(id)];try{window.focus();}catch(e){}if(n instanceof GoNotification){n.__fire('click');}};
try{if(window.ServiceWorkerRegistration){ServiceWorkerRegistration.prototype.showNotification=function(title,opts){opts=opts||{};post({id:'sw'+(++seq),title:String(title||''),body:String(opts.body||''),tag:String(opts.tag||'')});return Promise.resolve();};
ServiceWorkerRegistration.prototype.getNotifications=function(){return Promise.resolve([]);};}}catch(e){}
try{if(navigator.permissions&&navigator.permissions.query){var q=navigator.permissions.query.bind(navigator.permissions);
navigator.permissions.query=function(d){if(d&&d.name==='notifications'){return Promise.resolve({name:'notifications',state:'granted',onchange:null,addEventListener:function(){},removeEventListener:function(){}});}return q(d);};}}catch(e){}
})();}
// Unread count for the Dock badge / tray dot: "(N)" in the title, or unread
// rows in the conversation list, whichever is higher. Polled; posts on change.
if(location.hostname==='messages.google.com'){
var lastU=-1;
setInterval(function(){var n=0;
var m=(document.title||'').match(/\((\d+)\)/);if(m)n=parseInt(m[1],10);
try{var c=document.querySelectorAll('[data-e2e-is-unread="true"]').length;if(c>n)n=c;}catch(e){}
if(n!==lastU){var prefix=lastU<0?'unread-baseline:':'unread:';lastU=n;try{window.ipc.postMessage(prefix+n);}catch(e){}}},2000);
}
})();"##;
pub fn webview_bounds(window: &Window) -> wry::Rect {
    fullscreen_bounds(window, TOP_INSET)
}

pub fn fullscreen_bounds(window: &Window, top_inset: f64) -> wry::Rect {
    let scale = window.scale_factor();
    let size = window.inner_size();
    let y = (top_inset * scale) as u32;
    wry::Rect {
        position: wry::dpi::PhysicalPosition::new(0, y as i32).into(),
        size: wry::dpi::PhysicalSize::new(size.width, size.height.saturating_sub(y)).into(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn allows_product_and_signin() {
        assert!(navigation_allowed("about:blank"));
        assert!(navigation_allowed("https://messages.google.com/web"));
        assert!(navigation_allowed(
            "https://messages.google.com/web/conversations"
        ));
        assert!(navigation_allowed(
            "https://accounts.google.com/o/oauth2/v2/auth?x=1"
        ));
    }

    #[test]
    fn denies_the_rest() {
        assert!(!navigation_allowed("https://evil.com/"));
        assert!(!navigation_allowed("https://messages.google.com.evil.com/"));
        assert!(!navigation_allowed("https://fakemessages.google.com/"));
        assert!(!navigation_allowed("javascript:alert(1)"));
        assert!(!navigation_allowed("file:///etc/passwd"));
        assert!(!navigation_allowed("not a url"));
    }
}
