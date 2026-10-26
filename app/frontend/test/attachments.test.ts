// Unit tests for the message box's files. Run with `just unit`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { MAX_ATTACHMENTS, addAttachments, fileName } from "../src/lib/attachments.ts";

test("file names from macOS, Linux and Windows paths", () => {
  assert.equal(fileName("/Users/jb/Desktop/cat.jpg"), "cat.jpg");
  assert.equal(fileName("C:\\Users\\jb\\Desktop\\report.pdf"), "report.pdf");
  assert.equal(fileName(""), "");
});

test("dropped files add after the ones already there, once each", () => {
  const { files, dropped } = addAttachments(["/a.jpg"], ["/b.jpg", "/a.jpg", "", "/c.png", "/b.jpg"]);
  assert.deepEqual(files, ["/a.jpg", "/b.jpg", "/c.png"]);
  assert.equal(dropped, 0);
});

test("past the limit, the rest are counted, not added", () => {
  const many = Array.from({ length: MAX_ATTACHMENTS + 3 }, (_, i) => `/f${i}.jpg`);
  const { files, dropped } = addAttachments(["/f0.jpg"], many);
  assert.equal(files.length, MAX_ATTACHMENTS);
  assert.equal(files[MAX_ATTACHMENTS - 1], `/f${MAX_ATTACHMENTS - 1}.jpg`);
  assert.equal(dropped, 3);
});

test("the input list is left alone", () => {
  const before = ["/a.jpg"];
  addAttachments(before, ["/b.jpg"]);
  assert.deepEqual(before, ["/a.jpg"]);
});
