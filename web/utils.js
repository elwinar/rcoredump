import dayjs from "dayjs";

// dayjs is a lightweight momentjs-like library with mostly compatible API. I
// just need the UTC plugin to be able to handle timezones.
var utc = require("dayjs/plugin/utc");
dayjs.extend(utc);

// Format the date in a more friendly manner for display.
export function formatDate(date) {
  return dayjs(date).local().format("YYYY-MM-DD HH:mm:ss");
}

// Format a size in bytes into a human-readable string.
export function formatSize(bytes) {
  const threshold = 1000;
  const units = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"];
  let u = 0;
  while (Math.abs(bytes) >= threshold && u < units.length - 1) {
    bytes /= threshold;
    u += 1;
  }
  return bytes.toFixed(1) + " " + units[u];
}
//
// boolattr return the value for a non-HTML boolean attribute.
export function boolattr(b) {
  return b ? "true" : undefined;
}
