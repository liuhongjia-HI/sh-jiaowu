const { request } = require("../../utils/request");

Page({
  data: {
    course: {},
    materials: [],
    homework: [],
    stations: [],
    catalogUnits: [],
    lessonCount: 0,
    progress: 0,
    teacherText: "",
    materialCountText: "0 份资料",
    homeworkText: "可得徽章"
  },
  onLoad(options) {
    this.courseId = options.id || "";
    if (!this.courseId) {
      this.setData({ teacherText: "课程信息缺失" });
      return;
    }
    this.loadDetail();
  },
  onShareAppMessage() {
    return {
      title: this.data.course.name || "Starline 课程详情",
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
        catalogUnits: catalog.units,
        lessonCount: catalog.lessonCount,
        progress: data.progress || 0,
        teacherText:
          (materials[0] && materials[0].ownerTeacherName) ||
          (homework[0] && homework[0].ownerTeacherName) ||
          `${course.subject || ""}老师`,
        materialCountText: `${materials.length} 份资料`,
        homeworkText: homework.length ? `${homework.length} 个挑战` : "可得徽章"
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
  const units = (byParent.root || []).filter((node) => node.type === 'unit').map((unit) => ({
    ...unit,
    chapters: (byParent[unit.id] || []).filter((node) => node.type === 'chapter').map((chapter) => ({
      ...chapter,
      lessons: (byParent[chapter.id] || []).filter((node) => node.type === 'lesson').map((lesson) => {
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
          statusClass: status === '已完成' ? 'is-done' : (status === '学习中' ? 'is-active' : 'is-locked'),
          desc: count ? `${count} 项学习内容` : (status === '未开通' ? '开通后可查看讲义和练习' : '老师尚未发布内容'),
          materialId: (lessonMaterials[0] && lessonMaterials[0].id) || (active && active.materialId) || '',
          homeworkId: (lessonHomework[0] && lessonHomework[0].id) || (active && active.homeworkId) || ''
        };
      })
    }))
  }));
  return { units, lessonCount: units.reduce((sum, unit) => sum + unit.chapters.reduce((chapterSum, chapter) => chapterSum + chapter.lessons.length, 0), 0) };
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
