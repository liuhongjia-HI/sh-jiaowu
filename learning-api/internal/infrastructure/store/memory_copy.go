package store

import "starline/learning-api/internal/domain/learning"

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}

func cloneQuestion(value learning.Question) learning.Question {
	value.Options = cloneStrings(value.Options)
	value.Answers = cloneStrings(value.Answers)
	return value
}

func cloneQuestions(values []learning.Question) []learning.Question {
	out := make([]learning.Question, len(values))
	for index, value := range values {
		out[index] = cloneQuestion(value)
	}
	return out
}

func cloneQuestionBankItem(value learning.QuestionBankItem) learning.QuestionBankItem {
	value.Options = cloneStrings(value.Options)
	value.Answers = cloneStrings(value.Answers)
	return value
}

func cloneHomework(value learning.Homework) learning.Homework {
	value.QuestionIDs = cloneStrings(value.QuestionIDs)
	value.Questions = cloneQuestions(value.Questions)
	return value
}

func cloneSubmissionAnswers(values []learning.SubmissionAnswer) []learning.SubmissionAnswer {
	out := make([]learning.SubmissionAnswer, len(values))
	for index, value := range values {
		value.Choices = cloneStrings(value.Choices)
		out[index] = value
	}
	return out
}

func cloneSubmission(value learning.Submission) learning.Submission {
	value.Answers = cloneSubmissionAnswers(value.Answers)
	return value
}

func cloneCandidateStudent(value learning.CandidateStudent) learning.CandidateStudent {
	value.OpenedPackages = cloneStrings(value.OpenedPackages)
	return value
}

func cloneScheduleClass(value learning.ScheduleClass) learning.ScheduleClass {
	students := make([]learning.CandidateStudent, len(value.Students))
	for index, student := range value.Students {
		students[index] = cloneCandidateStudent(student)
	}
	value.Students = students
	return value
}
