const API_BASE = window.API_BASE || "http://localhost:8080/api";

const tbody = document.getElementById("novelTbody");
const searchInput = document.getElementById("searchInput");
const statusFilter = document.getElementById("statusFilter");
const refreshBtn = document.getElementById("refreshBtn");

async function loadNovels() {
  const q = encodeURIComponent(searchInput.value.trim());
  const status = encodeURIComponent(statusFilter.value);
  const res = await fetch(`${API_BASE}/novels?q=${q}&status=${status}`);
  const data = await res.json();

  tbody.innerHTML = "";
  data.forEach((n) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${escapeHtml(n.name)}</td>
      <td>${escapeHtml(n.platform || "-")}</td>
      <td>${statusText(n.status)}</td>
      <td>${n.rating}</td>
      <td>${n.url ? `<a href="${n.url}" target="_blank">查看</a>` : "-"}</td>
      <td><a class="btn" href="./edit.html?id=${n.id}">编辑</a></td>
    `;
    tbody.appendChild(tr);
  });
}

function statusText(status) {
  if (status === "unread") return "未读";
  if (status === "reading") return "在读";
  return "已读";
}

function escapeHtml(s) {
  return (s || "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[ch]));
}

refreshBtn.addEventListener("click", loadNovels);
searchInput.addEventListener("keydown", (e) => e.key === "Enter" && loadNovels());
statusFilter.addEventListener("change", loadNovels);

loadNovels().catch((err) => alert(`加载失败: ${err.message}`));
