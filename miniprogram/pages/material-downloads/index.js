const { request } = require("../../utils/request");
const statusLabels = { "准备中": "Queued", "打包中": "Preparing", "可下载": "Ready", "失败": "Failed", "已过期": "Expired" };
function sizeLabel(bytes) { return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${Math.ceil(bytes / 1024)} KB`; }
function decorateJobs(jobs, previous = []) {
  const expanded = new Map(previous.map(job => [job.id, job.expanded]));
  return jobs.map(job => ({ ...job, expanded: expanded.get(job.id) || false, materials: (job.materials || []).map(file => ({ ...file, pathText: [file.unit, file.chapter, file.lesson].filter(Boolean).join(" / ") })), statusText: statusLabels[job.status] || job.status, sizeText: sizeLabel(job.size), ready: job.status === "可下载", retryable: ["失败", "已过期"].includes(job.status), expiresText: job.expiresAt ? new Date(job.expiresAt).toLocaleString() : "" }));
}
Page({
  data: { loading: true, error: "", studentName: "", courses: [], groups: [], selectedCount: 0, selectedSize: "", jobs: [], generating: false, confirming: false, confirmed: false, pickup: null },
  onLoad(options) {
    this.courseId = options.courseId || "";
    this.challenge = options.challenge || "";
    this.selectedIDs = new Set();
    this.showing = true;
  },
  onShow() { this.showing = true; this.load(); },
  onHide() { this.showing = false; clearTimeout(this.pollTimer); },
  onUnload() { this.showing = false; this.unloaded = true; this.revision = (this.revision || 0) + 1; clearTimeout(this.pollTimer); },
  load() {
    clearTimeout(this.pollTimer);
    const revision = this.revision = (this.revision || 0) + 1;
    this.setData({ loading: true, error: "" });
    if (this.challenge) {
      this.setData({ pickup: null });
      return request(`/student/download-pickups/${encodeURIComponent(this.challenge)}`).then(pickup => {
        if (this.unloaded || revision !== this.revision) return;
        this.setData({ loading: false, pickup: { ...pickup, sizeText: sizeLabel(pickup.size) }, studentName: pickup.studentName });
      }).catch(error => { if (!this.unloaded && revision === this.revision) this.setData({ loading: false, error: error.message }); });
    }
    return Promise.all([request("/student/home", { silent: true }), request("/student/material-downloads", { silent: true })]).then(([home, jobs]) => {
      if (this.unloaded || revision !== this.revision) return;
      if (this.activeStudentId && this.activeStudentId !== (home.student || {}).id) { this.choices = null; this.selectedIDs = new Set(); this.setData({ groups: [], selectedCount: 0 }); }
      this.activeStudentId = (home.student || {}).id;
      this.setData({ studentName: (home.student || {}).name || "", loading: false, jobs: decorateJobs(jobs, this.data.jobs) });
      if (this.courseId && !this.choices) return this.loadChoices(revision);
    }).catch(error => { if (!this.unloaded && revision === this.revision) this.setData({ loading: false, error: error.message }); }).then(() => this.schedulePoll());
  },
  schedulePoll() {
    clearTimeout(this.pollTimer);
    if (this.showing && !this.unloaded && !this.challenge && this.data.jobs.some(job => ["准备中", "打包中"].includes(job.status))) this.pollTimer = setTimeout(() => this.refreshJobs(), 3000);
  },
  refreshJobs() {
    // Keep selections intact while polling task progress.
    const revision = this.revision;
    return request("/student/material-downloads", { silent: true }).then(jobs => {
      if (!this.showing || this.unloaded || revision !== this.revision) return;
      this.setData({ jobs: decorateJobs(jobs, this.data.jobs) });
    }).catch(() => {}).then(() => this.schedulePoll());
  },
  loadChoices(revision) {
    return request("/student/material-downloads/selection", { method: "POST", data: { courseIds: [this.courseId] }, silent: true }).then(quote => {
      if (this.unloaded || revision !== this.revision) return;
      this.choices = quote.materials || [];
      this.selectedIDs = new Set(this.choices.map(item => item.id));
      this.setData({ courses: quote.courses || [], studentName: quote.studentName || this.data.studentName });
      this.renderChoices();
    }).catch(error => { if (!this.unloaded && revision === this.revision) this.setData({ error: error.message }); });
  },
  renderChoices() {
    const groups = new Map();
    (this.choices || []).forEach(item => {
      const key = item.unit || "Course materials";
      if (!groups.has(key)) groups.set(key, { name: key, items: [] });
      groups.get(key).items.push({ ...item, checked: this.selectedIDs.has(item.id), sizeText: sizeLabel(item.size), pathText: [item.chapter, item.lesson].filter(Boolean).join(" / ") });
    });
    const selected = (this.choices || []).filter(item => this.selectedIDs.has(item.id));
    this.setData({ groups: [...groups.values()].map(group => {
      const chapters = new Map();
      group.items.forEach(item => { const name = item.chapter || "Materials"; if (!chapters.has(name)) chapters.set(name, { name, items: [] }); chapters.get(name).items.push(item); });
      return { ...group, checked: group.items.every(item => item.checked), chapters: [...chapters.values()].map(chapter => ({ ...chapter, checked: chapter.items.every(item => item.checked) })) };
    }), selectedCount: selected.length, selectedSize: sizeLabel(selected.reduce((sum, item) => sum + item.size, 0)) });
  },
  toggleItem(event) { const id = event.currentTarget.dataset.id; if (this.data.generating) return; this.selectedIDs.has(id) ? this.selectedIDs.delete(id) : this.selectedIDs.add(id); this.renderChoices(); },
  toggleGroup(event) { const group = this.data.groups[event.currentTarget.dataset.index]; if (!group || this.data.generating) return; group.items.forEach(item => group.checked ? this.selectedIDs.delete(item.id) : this.selectedIDs.add(item.id)); this.renderChoices(); },
  toggleChapter(event) { const { groupIndex, chapterIndex } = event.currentTarget.dataset; const group = this.data.groups[groupIndex]; const chapter = group && group.chapters[chapterIndex]; if (!chapter || this.data.generating) return; chapter.items.forEach(item => chapter.checked ? this.selectedIDs.delete(item.id) : this.selectedIDs.add(item.id)); this.renderChoices(); },
  toggleAll() { if (this.data.generating) return; const all = this.data.selectedCount === (this.choices || []).length; this.selectedIDs = new Set(all ? [] : (this.choices || []).map(item => item.id)); this.renderChoices(); },
  generate() {
    if (this.data.generating || !this.selectedIDs.size) return;
    this.setData({ generating: true });
    return request("/student/material-downloads", { method: "POST", data: { courseIds: [this.courseId], materialIds: [...this.selectedIDs] } }).then(() => { wx.showToast({ title: "Download task created", icon: "success" }); return this.refreshJobs(); }).catch(error => { if (!this.unloaded) { this.choices = null; this.setData({ error: error.message }); } }).then(() => { if (!this.unloaded) this.setData({ generating: false }); });
  },
  retryJob(event) { if (this.data.generating) return; this.setData({ generating: true }); return request(`/student/material-downloads/${encodeURIComponent(event.currentTarget.dataset.id)}/retry`, { method: "POST" }).then(() => this.refreshJobs()).catch(() => {}).then(() => { if (!this.unloaded) this.setData({ generating: false }); }); },
  copyLink(event) {
    const job = this.data.jobs.find(item => item.id === event.currentTarget.dataset.id && item.ready); if (!job) return;
    const origin = getApp().globalData.webBaseUrl.replace(/\/$/, "");
    wx.setClipboardData({ data: `${origin}/student-download?job=${encodeURIComponent(job.id)}`, success: () => wx.showModal({ title: "Link copied", content: "Send it to File Transfer on WeChat. Open it in a computer browser, then scan the QR code here to confirm.", showCancel: false }) });
  },
  showFiles(event) { const id = event.currentTarget.dataset.id; this.setData({ jobs: this.data.jobs.map(job => job.id === id ? { ...job, expanded: !job.expanded } : job) }); },
  scanPickup() { wx.scanCode({ onlyFromCamera: true, scanType: ["qrCode"], success: result => { const match = /^starline-download:([a-f0-9]{48})$/.exec(result.result || ""); if (!match) { wx.showToast({ title: "Use the QR code on the download page", icon: "none" }); return; } wx.navigateTo({ url: `/pages/material-downloads/index?challenge=${match[1]}` }); } }); },
  confirmPickup() {
    if (!this.challenge || !this.data.pickup || this.data.confirming || this.data.confirmed) return;
    this.setData({ confirming: true });
    return request(`/student/download-pickups/${encodeURIComponent(this.challenge)}/confirm`, { method: "POST" }).then(() => { if (!this.unloaded) this.setData({ confirmed: true }); }).catch(() => {}).then(() => { if (!this.unloaded) this.setData({ confirming: false }); });
  }
});
