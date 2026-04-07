const API_BASE = window.API_BASE || "http://localhost:8080/api";

const tbody = document.getElementById("novelTbody");
const searchInput = document.getElementById("searchInput");
const statusFilter = document.getElementById("statusFilter");
const refreshBtn = document.getElementById("refreshBtn");

async function loadNovels() {
  const q = encodeURIComponent(searchInput.value.trim());
  const status = encodeURIComponent(statusFilter.value);
  const res = await fetch(`${API_BASE}/novels?q=${q}&status=${status}`);
  if (!res.ok) throw new Error(await res.text());
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
      <td class="row-actions">
        <a class="btn" href="./edit.html?id=${n.id}">编辑</a>
        <button class="btn danger js-delete-btn" data-id="${n.id}" data-name="${escapeHtml(n.name)}" type="button">删除</button>
      </td>
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
tbody.addEventListener("click", async (e) => {
  const target = e.target;
  if (!(target instanceof HTMLElement) || !target.classList.contains("js-delete-btn")) return;
  const novelId = target.dataset.id;
  const novelName = target.dataset.name || "";
  if (!novelId) return;
  if (!confirm(`确定删除《${novelName || "该小说"}》吗？此操作不可恢复。`)) return;

  const res = await fetch(`${API_BASE}/novels/${novelId}`, { method: "DELETE" });
  if (!res.ok) {
    alert(`删除失败: ${await res.text()}`);
    return;
  }
  await loadNovels();
});

loadNovels().catch((err) => alert(`加载失败: ${err.message}`));
