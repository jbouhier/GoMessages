//! Stable display keys and monitor-relative window geometry.

use winit::monitor::MonitorHandle;
use winit::window::Window;

use super::prefs::{Geometry, MonitorGeometry};
use gomessages::config;

#[derive(Clone, Debug)]
pub struct Display {
    pub key: String,
    pub x: i32,
    pub y: i32,
    pub w: u32,
    pub h: u32,
    pub scale: f64,
}

impl Display {
    pub fn from_monitor(monitor: &MonitorHandle) -> Self {
        let position = monitor.position();
        let size = monitor.size();
        Self {
            key: monitor_key(monitor),
            x: position.x,
            y: position.y,
            w: size.width,
            h: size.height,
            scale: monitor.scale_factor().max(0.1),
        }
    }

    pub fn current(window: &Window) -> Option<Self> {
        window.current_monitor().as_ref().map(Self::from_monitor)
    }

    pub fn contains_center(&self, geometry: Geometry) -> bool {
        let cx = i64::from(geometry.x) + i64::from(geometry.w) / 2;
        let cy = i64::from(geometry.y) + i64::from(geometry.h) / 2;
        cx >= i64::from(self.x)
            && cx < i64::from(self.x) + i64::from(self.w)
            && cy >= i64::from(self.y)
            && cy < i64::from(self.y) + i64::from(self.h)
    }

    pub fn relative(&self, geometry: Geometry) -> MonitorGeometry {
        MonitorGeometry {
            x: ((i64::from(geometry.x) - i64::from(self.x)) as f64 / self.scale).round() as i32,
            y: ((i64::from(geometry.y) - i64::from(self.y)) as f64 / self.scale).round() as i32,
            w: (geometry.w as f64 / self.scale).round() as u32,
            h: (geometry.h as f64 / self.scale).round() as u32,
        }
    }

    pub fn absolute(&self, geometry: MonitorGeometry) -> Geometry {
        let margin = (16.0 * self.scale).round() as i64;
        let max_w = i64::from(self.w).saturating_sub(margin * 2).max(1);
        let max_h = i64::from(self.h).saturating_sub(margin * 2).max(1);
        let w = ((f64::from(geometry.w) * self.scale).round() as i64).clamp(1, max_w);
        let h = ((f64::from(geometry.h) * self.scale).round() as i64).clamp(1, max_h);
        let x = (i64::from(self.x) + (f64::from(geometry.x) * self.scale).round() as i64).clamp(
            i64::from(self.x) + margin,
            i64::from(self.x) + i64::from(self.w) - w - margin,
        );
        let y = (i64::from(self.y) + (f64::from(geometry.y) * self.scale).round() as i64).clamp(
            i64::from(self.y) + margin,
            i64::from(self.y) + i64::from(self.h) - h - margin,
        );
        Geometry {
            x: x as i32,
            y: y as i32,
            w: w as u32,
            h: h as u32,
        }
    }

    pub fn default_geometry(&self) -> Geometry {
        self.absolute(MonitorGeometry {
            x: ((self.w as f64 / self.scale - config::WINDOW_WIDTH) / 2.0).round() as i32,
            y: ((self.h as f64 / self.scale - config::WINDOW_HEIGHT) / 2.0).round() as i32,
            w: config::WINDOW_WIDTH as u32,
            h: config::WINDOW_HEIGHT as u32,
        })
    }
}

pub fn window_geometry(window: &Window) -> Option<Geometry> {
    let position = window.outer_position().ok()?;
    let size = window.inner_size();
    Some(Geometry {
        x: position.x,
        y: position.y,
        w: size.width,
        h: size.height,
    })
}

#[cfg(target_os = "macos")]
fn monitor_key(monitor: &MonitorHandle) -> String {
    use core_foundation::base::TCFType;
    use core_foundation::uuid::{CFUUIDGetUUIDBytes, CFUUIDRef, CFUUID};
    use winit::platform::macos::MonitorHandleExtMacOS;

    #[link(name = "ApplicationServices", kind = "framework")]
    unsafe extern "C" {
        fn CGDisplayCreateUUIDFromDisplayID(display: u32) -> CFUUIDRef;
    }
    let ptr = unsafe { CGDisplayCreateUUIDFromDisplayID(monitor.native_id()) };
    if !ptr.is_null() {
        let uuid = unsafe { CFUUID::wrap_under_create_rule(ptr) };
        let b = unsafe { CFUUIDGetUUIDBytes(uuid.as_concrete_TypeRef()) };
        let bytes = [
            b.byte0, b.byte1, b.byte2, b.byte3, b.byte4, b.byte5, b.byte6, b.byte7, b.byte8,
            b.byte9, b.byte10, b.byte11, b.byte12, b.byte13, b.byte14, b.byte15,
        ];
        return format!(
            "mac:{}",
            bytes.iter().map(|v| format!("{v:02x}")).collect::<String>()
        );
    }
    format!("mac-id:{}", monitor.native_id())
}

#[cfg(target_os = "windows")]
fn monitor_key(monitor: &MonitorHandle) -> String {
    use winit::platform::windows::MonitorHandleExtWindows;
    format!("windows:{}", monitor.native_id())
}

#[cfg(not(any(target_os = "macos", target_os = "windows")))]
fn monitor_key(monitor: &MonitorHandle) -> String {
    let size = monitor.size();
    format!(
        "display:{}:{}x{}",
        monitor.name().unwrap_or_default(),
        size.width,
        size.height
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn relative_geometry_follows_a_rearranged_monitor() {
        let first = Display {
            key: "one".into(),
            x: 0,
            y: 0,
            w: 2560,
            h: 1440,
            scale: 2.0,
        };
        let second = Display {
            key: "one".into(),
            x: -1920,
            y: 0,
            w: 1920,
            h: 1080,
            scale: 1.0,
        };
        let saved = first.relative(Geometry {
            x: 200,
            y: 100,
            w: 1800,
            h: 1000,
        });
        assert_eq!(
            saved,
            MonitorGeometry {
                x: 100,
                y: 50,
                w: 900,
                h: 500
            }
        );
        assert_eq!(
            second.absolute(saved),
            Geometry {
                x: -1820,
                y: 50,
                w: 900,
                h: 500
            }
        );
    }

    #[test]
    fn old_geometry_is_kept_on_screen() {
        let display = Display {
            key: "laptop".into(),
            x: 0,
            y: 0,
            w: 1280,
            h: 800,
            scale: 1.0,
        };
        let g = display.absolute(MonitorGeometry {
            x: 3000,
            y: -400,
            w: 2000,
            h: 1000,
        });
        assert_eq!(
            g,
            Geometry {
                x: 16,
                y: 16,
                w: 1248,
                h: 768
            }
        );
    }
}
