// Unit tests for linkify. Run with `just unit`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { isWebLink, linkify, mayPreview, shortURL } from "../src/lib/links.ts";

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

test("short addresses for chips", () => {
  assert.equal(shortURL("https://www.github.com/"), "github.com");
  assert.equal(shortURL("https://github.com/jb/repo/"), "github.com/jb/repo");
  assert.equal(shortURL("https://a.com/x?y=1"), "a.com/x?y=1");
  assert.equal(isWebLink("mailto:a@b.com"), false);
});

test("previews follow the policy and the sender", () => {
  assert.equal(mayPreview("contacts", false, "Haruto Sato"), true);
  assert.equal(mayPreview("contacts", false, "+1 555-123-0101"), false);
  assert.equal(mayPreview("contacts", false, "72345"), false);
  assert.equal(mayPreview("contacts", true, "+1 555-123-0101"), true);
  assert.equal(mayPreview("everyone", false, "+1 555-123-0101"), true);
  assert.equal(mayPreview("off", true, "Haruto Sato"), false);
  assert.equal(mayPreview("contacts", false, "Lucía Fernández"), true);
});
