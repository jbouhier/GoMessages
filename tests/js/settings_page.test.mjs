// Settings page script (src/bin/gomessages/settings.rs): banner + sound rows.

import { test, describe } from "node:test";
import assert from "node:assert/strict";
import { makeSettings } from "./harness.mjs";

describe("settings: alerts happy path", () => {
  test("defaults: banner and sound on, both previews enabled", () => {
    const s = makeSettings();
    assert.equal(s.els.popup.checked, true);
    assert.equal(s.els.popup.disabled, false);
    assert.equal(s.els.popuppreview.disabled, false);
    assert.equal(s.els.sound.checked, true);
    assert.equal(s.els.popuprow.hidden, false);
    assert.match(s.els.popuphint.textContent, /background/);
  });

  test("toggles send the exact messages Rust handles", () => {
    const s = makeSettings();
    s.els.popup.checked = false;
    s.trigger("popup", "onchange");
    s.els.popup.checked = true;
    s.trigger("popup", "onchange");
    s.trigger("popuppreview", "onclick");
    s.els.sound.checked = false;
    s.trigger("sound", "onchange");
    s.trigger("soundpreview", "onclick");
    assert.deepEqual(s.sent, [
      "popup:off",
      "popup:on",
      "popup-preview",
      "sound:off",
      "sound-preview",
    ]);
  });

  test("banner and sound render independently", () => {
    const s = makeSettings();
    s.setPrefs({ notification_popup: false, notification_sound: "system_notification" });
    assert.equal(s.els.popup.checked, false);
    assert.equal(s.els.sound.checked, true);
    assert.equal(s.els.popuppreview.disabled, true, "no test banner while off");
    s.setPrefs({ notification_popup: true, notification_sound: "off" });
    assert.equal(s.els.popup.checked, true);
    assert.equal(s.els.sound.checked, false);
    assert.equal(s.els.popuppreview.disabled, false);
  });
});

describe("settings: banner failure states", () => {
  test("denied by macOS: explains where to fix it", () => {
    const s = makeSettings({ prefs: { banner_status: "denied" } });
    assert.equal(s.els.popup.checked, true, "keeps the user's choice");
    assert.equal(s.els.popuppreview.disabled, true);
    assert.match(s.els.popuphint.textContent, /System Settings > Notifications/);
  });

  test("denied but turned off: no nagging", () => {
    const s = makeSettings({ prefs: { banner_status: "denied", notification_popup: false } });
    assert.doesNotMatch(s.els.popuphint.textContent, /Blocked/);
  });

  test("unsupported on macOS (not a bundled .app): disabled with a reason", () => {
    const s = makeSettings({ prefs: { banner_status: "unsupported" } });
    assert.equal(s.els.popuprow.hidden, false);
    assert.equal(s.els.popup.disabled, true);
    assert.equal(s.els.popup.checked, false);
    assert.equal(s.els.popuppreview.disabled, true);
    assert.ok(s.els.popuprow.classList.contains("off"));
    assert.match(s.els.popuphint.textContent, /bundled GoMessages\.app/);
  });

  for (const platform of ["linux", "windows"]) {
    test(`unsupported on ${platform}: row hidden, sound still works`, () => {
      const s = makeSettings({ platform, prefs: { banner_status: "unsupported" } });
      assert.equal(s.els.popuprow.hidden, true);
      assert.equal(s.els.sound.checked, true);
    });
  }
});

describe("settings: edge cases", () => {
  test("status answer arriving later updates the row", () => {
    const s = makeSettings({ prefs: { banner_status: "unknown" } });
    assert.equal(s.els.popuppreview.disabled, false);
    s.setPrefs({ banner_status: "denied" });
    assert.equal(s.els.popuppreview.disabled, true);
    s.setPrefs({ banner_status: "allowed" });
    assert.equal(s.els.popuppreview.disabled, false);
    assert.doesNotMatch(s.els.popuphint.textContent, /Blocked/);
  });

  test("leaving unsupported clears the dimmed state", () => {
    const s = makeSettings({ prefs: { banner_status: "unsupported" } });
    s.setPrefs({ banner_status: "allowed" });
    assert.equal(s.els.popuprow.classList.contains("off"), false);
    assert.equal(s.els.popup.disabled, false);
  });

  test("older Rust without banner fields: page still renders", () => {
    const s = makeSettings();
    assert.doesNotThrow(() =>
      s.ctx.__setPrefs({
        keep_running: true,
        tray: false,
        hide_dock: false,
        notification_sound: "system_notification",
        tray_ok: true,
      }),
    );
    assert.equal(s.els.popup.checked, false);
  });
});
