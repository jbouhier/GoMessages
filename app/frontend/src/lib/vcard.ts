// Contact cards (.vcf, vCard 3.0) for the info panel's Share.

export interface CardPerson {
  name: string;
  number: string;
  formatted_number?: string;
}

/** Escapes a vCard text value: backslash, comma, semicolon, newline. */
function esc(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/,/g, "\\,").replace(/;/g, "\\;").replace(/\r?\n/g, "\\n");
}

/** One card. N splits "First Last" at the last space so Contacts files it right. */
export function vcard(p: CardPerson): string {
  const name = p.name.trim() || p.formatted_number || p.number;
  const sp = name.lastIndexOf(" ");
  const n = sp > 0 && !/^\+?[\d\s()-]+$/.test(name) ? `${esc(name.slice(sp + 1))};${esc(name.slice(0, sp))};;;` : `;${esc(name)};;;`;
  const lines = ["BEGIN:VCARD", "VERSION:3.0", `FN:${esc(name)}`, `N:${n}`];
  if (p.number) lines.push(`TEL;TYPE=CELL:${p.number}`);
  lines.push("END:VCARD");
  return lines.join("\r\n") + "\r\n";
}

/** Several cards in one file (a group's members). */
export function vcards(people: CardPerson[]): string {
  return people.filter((p) => p.number).map(vcard).join("");
}

/** A file name from a contact or group name. */
export function cardFileName(name: string): string {
  const base = name.replace(/[\/\\:*?"<>|\x00-\x1f]/g, "").trim().slice(0, 80);
  return `${base || "Contact"}.vcf`;
}
