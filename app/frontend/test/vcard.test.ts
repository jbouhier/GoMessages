// Unit tests for contact cards. Run with `just unit`.
import { test } from "node:test";
import assert from "node:assert/strict";
import { cardFileName, vcard, vcards } from "../src/lib/vcard.ts";

test("a named contact", () => {
  assert.equal(
    vcard({ name: "Ada Lovelace", number: "+15551234567" }),
    "BEGIN:VCARD\r\nVERSION:3.0\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nTEL;TYPE=CELL:+15551234567\r\nEND:VCARD\r\n",
  );
});

test("a number-only contact stays whole", () => {
  assert.match(vcard({ name: "(555) 123-4567", number: "+15551234567" }), /\r\nN:;\(555\) 123-4567;;;\r\n/);
});

test("escapes special characters", () => {
  assert.match(vcard({ name: "Smith, John; Jr", number: "+1" }), /FN:Smith\\, John\\; Jr/);
});

test("group members, skipping ones without a number", () => {
  const out = vcards([{ name: "A", number: "+1" }, { name: "B", number: "" }, { name: "C", number: "+2" }]);
  assert.equal(out.match(/BEGIN:VCARD/g)?.length, 2);
});

test("file names", () => {
  assert.equal(cardFileName("Ada / Lovelace"), "Ada  Lovelace.vcf");
  assert.equal(cardFileName("  "), "Contact.vcf");
});
