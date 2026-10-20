// Unit tests for linkify. Run with `just unit`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { linkify } from "../src/lib/links.ts";

const hrefs = (t: string) => linkify(t).filter((s) => s.href).map((s) => s.href);

test("plain text has no links", () => {
  assert.deepEqual(linkify("hello there"), [{ text: "hello there" }]);
});

test("scheme, www and bare domains", () => {
  assert.deepEqual(hrefs("see https://a.com/x?y=1#z"), ["https://a.com/x?y=1#z"]);
  assert.deepEqual(hrefs("www.google.com"), ["https://www.google.com"]);
  assert.deepEqual(hrefs("go to github.com/jb/repo now"), ["https://github.com/jb/repo"]);
});

test("trailing punctuation and parens stay out", () => {
  assert.deepEqual(hrefs("Look: https://a.com."), ["https://a.com"]);
  assert.deepEqual(hrefs("(see https://a.com/x)"), ["https://a.com/x"]);
  assert.deepEqual(hrefs("https://en.wikipedia.org/wiki/Foo_(bar)"), ["https://en.wikipedia.org/wiki/Foo_(bar)"]);
});

test("emails become mailto", () => {
  assert.deepEqual(hrefs("mail jb@example.com"), ["mailto:jb@example.com"]);
});

test("file names and versions stay text", () => {
  assert.deepEqual(hrefs("open notes.txt, on v0.4.0, 3.14"), []);
});

test("segments rebuild the original text", () => {
  const t = "a https://x.io b www.y.com. c";
  assert.equal(linkify(t).map((s) => s.text).join(""), t);
});
