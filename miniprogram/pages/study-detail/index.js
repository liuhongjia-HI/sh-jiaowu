const { request } = require("../../utils/request");

const STATUS_LABELS = {
  "学习中": "Learning",
  "已完成": "Completed",
  "待挑战": "Ready",
  "未开通": "Locked",
  "未解锁": "Locked",
  "暂无内容": "No content"
};

Page({
  data: {
    course: {},
    materials: [],
    homework: [],
    stations: [],
    catalogLessons: [],
    lessonCount: 0,
    materialCountText: "0 materials",
    homeworkText: "Earn badges"
  },
  onLoad(options) {
    this.courseId = options.id || "";
    if (!this.courseId) {
      return;
    }
    this.loadDetail();
  },
  onShareAppMessage() {
    return {
      title: this.data.course.name || "Starline course details",
      path: this.courseId ? `/pages/study-detail/index?id=${encodeURIComponent(this.courseId)}` : "/pages/study/index"
    };
  },
  onShow() {
    // 从答题页提交返回时刷新站点状态
    if (this.courseId && this.loaded) {
      this.loadDetail();
    }
  },
  goBack() {
    wx.navigateBack({ delta: 1 });
  },
  loadDetail() {
    request(`/student/study/${this.courseId}`).then((data) => {
      const course = data.course || {};
      const materials = data.materials || [];
      const homework = data.homework || [];
      this.loaded = true;
      const stations = (data.stations || []).map(decorateStation);
      const catalog = buildCatalog(course.curriculum || [], stations, materials, homework);
      this.setData({
        course,
        materials,
        homework,
        stations,
        catalogLessons: catalog,
        lessonCount: catalog.length,
        materialCountText: `${materials.length} materials`,
        homeworkText: homework.length ? `${homework.length} exercises` : "Earn badges"
      });
    });
  },
  tapLesson(event) {
    const { status, lessonId, materialId, homeworkId } = event.currentTarget.dataset;
    if (status === "未开通" || status === "未解锁" || status === "暂无内容") return;
    if (materialId) {
      wx.navigateTo({ url: `/pages/material-preview/index?id=${materialId}&courseId=${encodeURIComponent(this.courseId)}&lessonId=${encodeURIComponent(lessonId || '')}` });
      return;
    }
    if (homeworkId) {
      wx.navigateTo({ url: `/pages/material-preview/index?courseId=${encodeURIComponent(this.courseId)}&lessonId=${encodeURIComponent(lessonId || '')}` });
    }
  }
});

function decorateStation(item) {
  const status = item.status || "未解锁";
  return {
    ...item,
    statusClass: status === "已完成" ? "is-done" : status === "学习中" ? "is-active" : "is-locked"
  };
}

function buildCatalog(nodes, stations, materials, homework) {
  const byParent = {};
  (nodes || []).forEach((node) => {
    const key = node.parentId || 'root';
    if (!byParent[key]) byParent[key] = [];
    byParent[key].push(node);
  });
  Object.keys(byParent).forEach((key) => byParent[key].sort(compareNode));
  const stationByLesson = groupByLesson(stations);
  const materialByLesson = groupByLesson(materials);
  const homeworkByLesson = groupByLesson(homework);
  const orderedLessons = [];
  const walk = (parentId, ancestors, orderPath) => {
    (byParent[parentId] || []).forEach((node) => {
      const children = byParent[node.id] || [];
      if (!children.length) {
        const parentName = ancestors.length ? ancestors[ancestors.length - 1] : '';
        const number = [...orderPath, node.sortOrder || 1].join('.');
        orderedLessons.push({ ...node, displayName: `${number} · ${parentName ? `${parentName} · ` : ''}${node.name}` });
        return;
      }
      walk(node.id, node.name ? [...ancestors, node.name] : ancestors, [...orderPath, node.sortOrder || 1]);
    });
  };
  walk('root', [], []);
  return orderedLessons.map((lesson) => {
    const lessonStations = stationByLesson[lesson.id] || [];
    const lessonMaterials = materialByLesson[lesson.id] || [];
    const lessonHomework = homeworkByLesson[lesson.id] || [];
    const active = lessonStations.find((item) => item.status === '学习中') || lessonStations.find((item) => item.status === '已完成') || lessonStations.find((item) => item.status === '待挑战');
    const locked = lessonStations.some((item) => item.status === '未开通' || item.status === '未解锁');
    const count = lessonMaterials.length + lessonHomework.length;
    const contentStatus = lessonMaterials.length ? '学习中' : (lessonHomework.length ? '待挑战' : '');
    const status = active ? active.status : (contentStatus || (locked ? '未开通' : '暂无内容'));
    return {
      ...lesson,
      icon: status === '未开通' ? '🔒' : (status === '暂无内容' ? '·' : '📖'),
      status,
      statusLabel: STATUS_LABELS[status] || status,
      statusClass: status === '已完成' ? 'is-done' : (status === '学习中' ? 'is-active' : 'is-locked'),
      desc: count ? `${count} learning item${count === 1 ? '' : 's'}` : (status === '未开通' ? 'Unlock to view materials and exercises' : "Teacher hasn't published content yet"),
      materialId: (lessonMaterials[0] && lessonMaterials[0].id) || (active && active.materialId) || '',
      homeworkId: (lessonHomework[0] && lessonHomework[0].id) || (active && active.homeworkId) || ''
    };
  });
}

function groupByLesson(items) {
  return (items || []).reduce((result, item) => {
    if (!item.lessonId) return result;
    if (!result[item.lessonId]) result[item.lessonId] = [];
    result[item.lessonId].push(item);
    return result;
  }, {});
}

function compareNode(left, right) {
  return (left.sortOrder || 0) - (right.sortOrder || 0);
}
