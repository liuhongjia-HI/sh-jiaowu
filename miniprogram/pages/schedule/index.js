const { request } = require("../../utils/request");

const weekOptions = [
  { label: "Mon", value: 1 },
  { label: "Tue", value: 2 },
  { label: "Wed", value: 3 },
  { label: "Thu", value: 4 },
  { label: "Fri", value: 5 },
  { label: "Sat", value: 6 },
  { label: "Sun", value: 7 }
];

Page({
  data: {
    loading: true,
    loadError: "",
    availability: [],
    classes: [],
    nextClass: null,
    weekOptions,
    weekFallback: "Select day"
  },
  onLoad() {
    this.loadData();
  },
  onShareAppMessage() {
    return {
      title: "Starline Schedule",
      path: "/pages/schedule/index"
    };
  },
  loadData() {
    Promise.all([
      request("/student/availability"),
      request("/student/schedule")
    ])
      .then(([availability, classes]) => {
        const confirmedClasses = classes.map(withClassDisplay).filter(isConfirmedClass);
        this.setData({
          availability: availability.map(withWeekLabel),
          classes: confirmedClasses,
          nextClass: confirmedClasses[0] || null,
          loadError: "",
          loading: false
        });
      })
      .catch(() => {
        this.setData({ loading: false, loadError: "Failed to load schedule. Try again later." });
        wx.showToast && wx.showToast({ title: "Failed to load schedule", icon: "none" });
      });
  },
  addSlot() {
    const availability = this.data.availability.concat({
      dayOfWeek: 3,
      weekLabel: "Wed",
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
      wx.showToast({ title: "Enter a valid time range", icon: "none" });
      return;
    }
    request("/student/availability", {
      method: "PUT",
      data: { slots: this.data.availability.map(({ weekLabel, ...slot }) => slot) }
    }).then((availability) => {
      this.setData({ availability: availability.map(withWeekLabel) });
      wx.showToast({ title: "Saved", icon: "success" });
    });
  }
});

function getWeekLabel(day) {
  const option = weekOptions.find((item) => item.value === day);
  return option ? option.label : "";
}

function withWeekLabel(slot) {
  return { ...slot, weekLabel: getWeekLabel(slot.dayOfWeek) || "Select day" };
}

function withClassDisplay(item) {
  const weekLabel = getWeekLabel(item.dayOfWeek);
  return {
    ...item,
    weekLabel,
    timeText: [weekLabel, `${item.startTime || ""}-${item.endTime || ""}`].filter(Boolean).join(" "),
    periodText: formatPeriod(item.startDate, item.endDate),
    statusText: item.status === "已确认" || !item.status ? "Confirmed" : item.status
  };
}

function isConfirmedClass(item) {
  return !item.status || item.status === "已确认";
}

function formatPeriod(startDate, endDate) {
  if (startDate && endDate) {
    return `${startDate} to ${endDate}`;
  }
  if (startDate) {
    return `${startDate} onwards`;
  }
  if (endDate) {
    return `Until ${endDate}`;
  }
  return "Regular schedule";
}

function isValidTime(value) {
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(value || "");
}
