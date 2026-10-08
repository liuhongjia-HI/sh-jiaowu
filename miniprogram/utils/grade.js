const GRADES = ["一", "二", "三", "四", "五", "六", "七", "八", "九", "十", "十一", "十二"];

function gradeLabel(value) {
  const grade = String(value || "").trim();
  const match = grade.match(/^(.+)年级$/);
  if (!match) return grade;
  const index = GRADES.indexOf(match[1]);
  if (index >= 0) return `Grade ${index + 1}`;
  const number = Number(match[1]);
  return Number.isInteger(number) && number >= 1 && number <= 12 ? `Grade ${number}` : grade;
}

module.exports = { gradeLabel };
