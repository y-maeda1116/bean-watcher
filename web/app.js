// bean-watcher ダッシュボード: data.json / config.json を fetch して表示する。
// 書き込みは行わない（閲覧専用）。入力は GitHub Actions の record ワークフロー。

async function loadJSON(path) {
  const res = await fetch(path, { cache: "no-store" });
  if (!res.ok) throw new Error(path + ": " + res.status);
  return res.json();
}

function levelBadge(level) {
  const map = {
    OK: { text: "OK", cls: "ok" },
    LOW: { text: "買い時", cls: "low" },
    CRITICAL: { text: "もうすぐ切れる", cls: "critical" },
    DUE: { text: "掃除の目安", cls: "critical" },
    "": { text: "未設定", cls: "pending" },
  };
  return map[level] || { text: level || "—", cls: "pending" };
}

function fmt(n, d) {
  if (n === null || n === undefined || Number.isNaN(n)) return "—";
  return Number(n).toFixed(d === undefined ? 1 : d);
}

function clearChildren(el) {
  while (el.firstChild) el.removeChild(el.firstChild);
}

function renderChart(records) {
  const el = document.getElementById("chart");
  clearChildren(el);
  if (!records || records.length === 0) {
    el.textContent = "記録がありません";
    return;
  }
  const sorted = records.slice().sort(function (a, b) { return a.date.localeCompare(b.date); });
  const max = Math.max.apply(null, sorted.map(function (r) { return r.cups; }).concat([1]));
  sorted.forEach(function (r) {
    const bar = document.createElement("div");
    bar.className = "bar";
    const fill = document.createElement("div");
    fill.className = "bar-fill";
    fill.style.height = (r.cups / max) * 100 + "%";
    const label = document.createElement("span");
    label.className = "bar-label";
    label.textContent = r.date.slice(5) + ":" + r.cups;
    bar.appendChild(fill);
    bar.appendChild(label);
    el.appendChild(bar);
  });
}

function renderPurchases(purchases) {
  const el = document.getElementById("purchases");
  clearChildren(el);
  if (!purchases || purchases.length === 0) {
    const li = document.createElement("li");
    li.textContent = "まだ購入記録がありません";
    el.appendChild(li);
    return;
  }
  purchases.slice().reverse().forEach(function (p) {
    const li = document.createElement("li");
    li.textContent = p.date + "  " + p.bags + "袋 (" + p.grams + "g)";
    el.appendChild(li);
  });
}

function maintLine(level, days, shots) {
  const b = levelBadge(level);
  return b.text + "  経過" + days + "日 / " + shots + "杯";
}

function applyBadge(el, level) {
  const b = levelBadge(level);
  el.textContent = b.text;
  el.className = "badge " + b.cls;
}

async function main() {
  const status = document.getElementById("status");
  try {
    const data = await loadJSON("data.json");
    const s = data.summary || {};

    status.classList.add("hidden");
    document.querySelectorAll(".card.hidden").forEach(function (el) { el.classList.remove("hidden"); });

    document.getElementById("updated").textContent = s.updated_at ? "更新: " + s.updated_at : "";

    const grams = (s.remaining_grams !== undefined) ? s.remaining_grams : (data.beans ? data.beans.remaining_grams : 0);
    document.getElementById("beans-remaining").textContent =
      Math.round(grams) + "g" + (s.remaining_bags !== undefined ? " / " + fmt(s.remaining_bags) + "袋" : "");
    applyBadge(document.getElementById("beans-badge"), s.beans_level);
    document.getElementById("beans-predict").textContent =
      "あと約 " + fmt(s.predicted_days) + "日（1日平均 " + fmt(s.avg_cups_per_day) + "杯）";

    renderChart(data.usage ? data.usage.daily_records : []);
    renderPurchases(data.purchases || []);

    document.getElementById("descaling-status").textContent =
      maintLine(s.descaling_level, s.descaling_days_elapsed, s.descaling_shots_elapsed);
    document.getElementById("grinder-status").textContent =
      maintLine(s.grinder_level, s.grinder_days_elapsed, s.grinder_shots_elapsed);
  } catch (e) {
    status.classList.remove("hidden");
    status.classList.add("error");
    status.querySelector("p").textContent = "データを読み込めませんでした: " + e.message;
  }
}

main();

// 「買った袋数メモ」: localStorage にだけ保存する入力欄（本番 data.json には反映しない）。
const MEMO_KEY = "bean-watcher:memo";

function loadMemo() {
  try {
    const raw = localStorage.getItem(MEMO_KEY);
    const list = raw ? JSON.parse(raw) : [];
    return Array.isArray(list) ? list : [];
  } catch (e) {
    return [];
  }
}

function saveMemo(list) {
  try {
    localStorage.setItem(MEMO_KEY, JSON.stringify(list));
  } catch (e) {
    // localStorage が利用できない環境（プライベートモード等）では何もしない
  }
}

function fmtMemoDate(iso) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("ja-JP", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function renderMemo() {
  const el = document.getElementById("memo-list");
  const clearBtn = document.getElementById("memo-clear");
  if (!el) return;
  clearChildren(el);
  const list = loadMemo();
  if (clearBtn) clearBtn.style.display = list.length === 0 ? "none" : "";
  if (list.length === 0) {
    const li = document.createElement("li");
    li.textContent = "まだ記録がありません";
    el.appendChild(li);
    return;
  }
  list.forEach(function (item, i) {
    const li = document.createElement("li");
    li.className = "memo-item";
    const span = document.createElement("span");
    span.textContent = fmtMemoDate(item.date) + "  " + item.bags + "袋";
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "link-btn";
    btn.textContent = "削除";
    btn.addEventListener("click", function () { deleteMemo(i); });
    li.appendChild(span);
    li.appendChild(btn);
    el.appendChild(li);
  });
}

function addMemo(bags) {
  const list = loadMemo();
  const next = [{ bags: bags, date: new Date().toISOString() }].concat(list);
  saveMemo(next);
  renderMemo();
}

function deleteMemo(index) {
  const next = loadMemo().filter(function (_, i) { return i !== index; });
  saveMemo(next);
  renderMemo();
}

function clearMemo() {
  saveMemo([]);
  renderMemo();
}

function initMemo() {
  const input = document.getElementById("memo-bags");
  const addBtn = document.getElementById("memo-add");
  const clearBtn = document.getElementById("memo-clear");
  if (!input || !addBtn) return;

  const submit = function () {
    const v = parseInt(input.value, 10);
    if (!Number.isFinite(v) || v <= 0) {
      input.focus();
      return;
    }
    addMemo(v);
    input.value = "";
    input.focus();
  };

  addBtn.addEventListener("click", submit);
  input.addEventListener("keydown", function (e) {
    if (e.key === "Enter") submit();
  });
  if (clearBtn) clearBtn.addEventListener("click", clearMemo);
  renderMemo();
}

initMemo();
