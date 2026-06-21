const { request } = require("../../utils/request");

const weekOptions = [
  { label: "周一", value: 1 },
  { label: "周二", value: 2 },
  { label: "周三", value: 3 },
  { label: "周四", value: 4 },
  { label: "周五", value: 5 },
  { label: "周六", value: 6 },
  { label: "周日", value: 7 }
];

Page({
  data: {
    loading: true,
    availability: [],
    classes: [],
    weekOptions,
    weekFallback: "选择星期"
  },
  onLoad() {
    this.loadData();
  },
  loadData() {
    Promise.all([
      request("/student/availability"),
      request("/student/schedule")
    ])
      .then(([availability, classes]) => {
        this.setData({
          availability: availability.map(withWeekLabel),
          classes: classes.map(withClassWeekLabel),
          loading: false
        });
      })
      .catch(() => this.setData({ loading: false }));
  },
  addSlot() {
    const availability = this.data.availability.concat({
      dayOfWeek: 3,
      weekLabel: "周三",
      startTime: "19:00",
      endTime: "20:30"
    });
    this.setData({ availability });
  },
  removeSlot(event) {
    const index = Number(event.currentTarget.dataset.index);
    const availability = this.data.availability.filter((_, itemIndex) => itemIndex !== index);
    this.setData({ availability });
  },
  changeWeek(event) {
    const index = Number(event.currentTarget.dataset.index);
    const value = weekOptions[Number(event.detail.value)].value;
    const availability = this.data.availability.slice();
    availability[index] = withWeekLabel({ ...availability[index], dayOfWeek: value });
    this.setData({ availability });
  },
  changeStart(event) {
    this.updateSlot(event.currentTarget.dataset.index, "startTime", event.detail.value);
  },
  changeEnd(event) {
    this.updateSlot(event.currentTarget.dataset.index, "endTime", event.detail.value);
  },
  updateSlot(index, key, value) {
    index = Number(index);
    const availability = this.data.availability.slice();
    availability[index] = { ...availability[index], [key]: value };
    this.setData({ availability });
  },
  saveAvailability() {
    const invalid = this.data.availability.some((slot) => !isValidTime(slot.startTime) || !isValidTime(slot.endTime) || slot.startTime >= slot.endTime);
    if (invalid) {
      wx.showToast({ title: "请填写正确的时间段", icon: "none" });
      return;
    }
    request("/student/availability", {
      method: "PUT",
      data: { slots: this.data.availability.map(({ weekLabel, ...slot }) => slot) }
    }).then((availability) => {
      this.setData({ availability: availability.map(withWeekLabel) });
      wx.showToast({ title: "已保存", icon: "success" });
    });
  }
});

function getWeekLabel(day) {
  const option = weekOptions.find((item) => item.value === day);
  return option ? option.label : "";
}

function withWeekLabel(slot) {
  return { ...slot, weekLabel: getWeekLabel(slot.dayOfWeek) || "选择星期" };
}

function withClassWeekLabel(item) {
  return { ...item, weekLabel: getWeekLabel(item.dayOfWeek) };
}

function isValidTime(value) {
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(value || "");
}
