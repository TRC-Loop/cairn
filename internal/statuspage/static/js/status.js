// SPDX-License-Identifier: AGPL-3.0-or-later
// cairn · public status page poller.
// Vanilla JS. No framework, no external deps. ~60 lines.
(function () {
  "use strict";

  var root = document.querySelector("main.shell");
  if (!root) return;

  var slug = root.getAttribute("data-slug");
  if (!slug) return;

  var POLL_MS = 30000;
  var endpoint = "/p/" + encodeURIComponent(slug) + "/api.json";

  function pretty(status) {
    switch (status) {
      case "operational":    return "All systems operational";
      case "degraded":       return "Degraded performance";
      case "partial_outage": return "Partial outage";
      case "major_outage":   return "Major outage";
      case "maintenance":    return "Scheduled maintenance";
      default:               return "Status unknown";
    }
  }

  function apply(data) {
    if (!data || typeof data !== "object") return;
    document.documentElement.setAttribute("data-overall", data.overall_status || "unknown");

    var head = document.querySelector(".masthead");
    if (head) {
      head.classList.remove("s-up", "s-degraded", "s-down", "s-maintenance", "s-unknown");
      head.classList.add(mapClass(data.overall_status));
    }
    var headline = document.querySelector(".masthead .headline");
    if (headline) {
      var txt = headline.lastChild;
      if (txt && txt.nodeType === Node.TEXT_NODE) txt.textContent = " " + pretty(data.overall_status);
      else headline.appendChild(document.createTextNode(" " + pretty(data.overall_status)));
    }

    if (Array.isArray(data.components)) {
      data.components.forEach(function (c) {
        var row = document.querySelector('[data-component-id="' + c.id + '"]');
        if (!row) return;
        row.setAttribute("data-status", c.status);
        var dot = row.querySelector(".component-status-dot");
        if (dot) { resetStatusClass(dot); dot.classList.add(mapClass(c.status)); }
        var label = row.querySelector(".component-status-label");
        if (label) label.textContent = friendly(c.status);
        var mark = row.querySelector(".component-mark");
        if (mark) { resetStatusClass(mark); mark.classList.add(mapClass(c.status)); }
      });
    }

    var live = document.querySelector('time[data-live="true"]');
    if (live && data.page && data.page.updated_at) live.setAttribute("datetime", data.page.updated_at);
  }

  function mapClass(s) {
    if (s === "up" || s === "operational") return "s-up";
    if (s === "degraded") return "s-degraded";
    if (s === "down" || s === "major_outage" || s === "partial_outage") return "s-down";
    if (s === "maintenance") return "s-maintenance";
    return "s-unknown";
  }
  function resetStatusClass(el) {
    el.classList.remove("s-up", "s-degraded", "s-down", "s-maintenance", "s-unknown", "s-nodata");
  }
  function friendly(s) {
    if (s === "up") return "Operational";
    if (s === "degraded") return "Degraded";
    if (s === "down") return "Down";
    if (s === "maintenance") return "Maintenance";
    return "Unknown";
  }

  function poll() {
    fetch(endpoint, { headers: { "Accept": "application/json" }, credentials: "same-origin" })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (data) { if (data) apply(data); })
      .catch(function () { /* ignore transient network errors */ });
  }

  if (document.visibilityState !== "hidden") setInterval(poll, POLL_MS);
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") poll();
  });
})();
