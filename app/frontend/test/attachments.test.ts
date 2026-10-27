// Unit tests for the message box's files. Run with `just unit`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { MAX_ATTACHMENTS, addAttachments, attachmentKind, attachmentURL, fileName } from "../src/lib/attachments.ts";

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

test("pictures and videos preview, other files show their name", () => {
  assert.equal(attachmentKind("/x/IMG_0001.HEIC"), "image");
  assert.equal(attachmentKind("C:\\x\\photo.jpeg"), "image");
  assert.equal(attachmentKind("/x/clip.MOV"), "video");
  assert.equal(attachmentKind("/x/clip.webm"), "video");
  assert.equal(attachmentKind("/x/report.pdf"), "file");
  assert.equal(attachmentKind("/x/png"), "file", "a name without an extension");
  assert.equal(attachmentKind("/x.jpg/notes"), "file", "a folder's extension doesn't count");
});

test("the preview address carries the whole path", () => {
  assert.equal(attachmentURL("C:\\a b\\c&d.jpg"), "/attachment?path=C%3A%5Ca%20b%5Cc%26d.jpg");
});
