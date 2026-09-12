const SUBJECT_ENGLISH_NAMES = {
  英文: "English",
  英语: "English",
  English: "English",
  数学: "Mathematics",
  Math: "Mathematics",
  Mathematics: "Mathematics",
  语文: "Chinese",
  Chinese: "Chinese",
  科学: "Science",
  Science: "Science",
  综合科学: "Integrated Science",
  "Integrated Science": "Integrated Science",
  地理: "Geography",
  Geography: "Geography",
  历史: "History",
  History: "History",
  物理: "Physics",
  Physics: "Physics",
  化学: "Chemistry",
  Chemistry: "Chemistry"
};

const SUBJECT_KEYS = {
  英文: "english",
  英语: "english",
  English: "english",
  数学: "math",
  Math: "math",
  Mathematics: "math",
  语文: "chinese",
  Chinese: "chinese",
  科学: "science",
  Science: "science",
  综合科学: "integrated-science",
  "Integrated Science": "integrated-science",
  地理: "geography",
  Geography: "geography",
  历史: "history",
  History: "history",
  物理: "physics",
  Physics: "physics",
  化学: "chemistry",
  Chemistry: "chemistry"
};

function subjectLabel(subject) {
  const name = String(subject || "").trim();
  return SUBJECT_ENGLISH_NAMES[name] || name;
}

function subjectKey(subject) {
  const name = String(subject || "").trim();
  return SUBJECT_KEYS[name] || name.toLowerCase();
}

function subjectsMatchName(left, right) {
  const first = String(left || "").trim();
  const second = String(right || "").trim();
  if (first === second) return true;
  if (!first || !second) return false;
  return subjectKey(first) === subjectKey(second);
}

function subjectEmoji(subject, index) {
  const icons = {
    数学: "➗",
    Mathematics: "➗",
    Math: "➗",
    英文: "🔤",
    英语: "🔤",
    English: "🔤",
    语文: "📖",
    Chinese: "📖",
    科学: "🔬",
    Science: "🔬",
    综合科学: "🔬",
    地理: "🌍",
    Geography: "🌍",
    历史: "📜",
    History: "📜",
    物理: "⚙️",
    Physics: "⚙️",
    化学: "🧪",
    Chemistry: "🧪"
  };
  return icons[String(subject || "").trim()] || (index % 2 === 0 ? "📚" : "✨");
}

module.exports = {
  subjectLabel,
  subjectsMatchName,
  subjectEmoji
};
