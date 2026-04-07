const API_BASE = window.API_BASE || "http://localhost:8080/api";
const form = document.getElementById("novelForm");
const deleteBtn = document.getElementById("deleteBtn");
const params = new URLSearchParams(window.location.search);
const id = params.get("id");

if (id) {
  deleteBtn.hidden = false;
  loadNovel(id).catch((err) => alert(`加载失败: ${err.message}`));
}

async function loadNovel(novelId) {
  const res = await fetch(`${API_BASE}/novels/${novelId}`);
  if (!res.ok) throw new Error(await res.text());
  const n = await res.json();
  form.name.value = n.name || "";
  form.platform.value = n.platform || "";
  form.url.value = n.url || "";
  form.file.value = n.file || "";
  form.description.value = n.description || "";
  form.status.value = n.status || "unread";
  form.rating.value = n.rating ?? 0;
}

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  const payload = {
    name: form.name.value.trim(),
    platform: form.platform.value.trim(),
    url: form.url.value.trim(),
    file: form.file.value.trim(),
    description: form.description.value.trim(),
    status: form.status.value,
    rating: Number(form.rating.value || 0),
  };

  const method = id ? "PUT" : "POST";
  const path = id ? `/novels/${id}` : "/novels";
  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    alert(`保存失败: ${await res.text()}`);
    return;
  }
  alert("保存成功");
  window.location.href = "./index.html";
});

deleteBtn.addEventListener("click", async () => {
  if (!confirm("确定删除该小说？")) return;
  const res = await fetch(`${API_BASE}/novels/${id}`, { method: "DELETE" });
  if (!res.ok) {
    alert(`删除失败: ${await res.text()}`);
    return;
  }
  alert("删除成功");
  window.location.href = "./index.html";
});
