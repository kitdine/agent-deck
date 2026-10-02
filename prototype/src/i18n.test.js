import assert from "node:assert/strict";
import test from "node:test";
import { formatHourWindow } from "./i18n.js";

// Independent strings from the native hour().minute() contract, including rollover.
for (const [lang, hour, expected] of [
  ["en", 0, "12:00\u202fAM–1:00\u202fAM"],
  ["en", 15, "3:00\u202fPM–4:00\u202fPM"],
  ["en", 23, "11:00\u202fPM–12:00\u202fAM"],
  ["zh", 0, "00:00–01:00"],
  ["zh", 15, "15:00–16:00"],
  ["zh", 23, "23:00–00:00"],
]) {
  test(`${lang} one-hour window at ${hour}`, () => {
    assert.equal(formatHourWindow(hour, lang), expected);
  });
}
