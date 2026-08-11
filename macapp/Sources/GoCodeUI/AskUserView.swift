import HarnessKit
import SwiftUI

/// Renders and submits a structured question requested by the running agent.
struct AskUserView: View {
    let prompt: AskUserPrompt
    let answerInFlight: Bool
    let onAnswer: ([String: String]) -> Void
    @State private var answers: [String: String] = [:]

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.inset) {
            Label("The agent needs your input", systemImage: "questionmark.bubble")
                .font(Typography.body.weight(.medium))

            ForEach(prompt.questions) { question in
                QuestionAnswerField(question: question, answers: $answers)
            }

            HStack {
                if let deadline = prompt.deadlineAt {
                    Text("Answer by \(deadline.formatted(date: .omitted, time: .shortened))")
                        .font(Typography.caption)
                        .foregroundStyle(Theme.foregroundTertiary)
                }
                Spacer()
                Button("Send") { onAnswer(answers) }
                    .buttonStyle(.borderedProminent)
                    .disabled(
                        !AskUserAnswers.isComplete(prompt: prompt, answers: answers)
                            || answerInFlight)
            }
        }
        // Same 16pt left inset as the transcript column and the status bar.
        .padding(.horizontal, Spacing.large)
        .padding(.vertical, 14)
        .background(Theme.accent.opacity(StateOpacity.subtle))
    }
}

private struct QuestionAnswerField: View {
    let question: AskUserQuestion
    @Binding var answers: [String: String]

    var body: some View {
        VStack(alignment: .leading, spacing: Spacing.small) {
            Text(question.question).font(Typography.body)
            if question.isFreeform {
                TextField("Your answer", text: answerBinding)
            } else {
                ForEach(question.options ?? [], id: \.label) { option in
                    OptionAnswerButton(
                        option: option,
                        isSelected: answers[question.id] == option.label
                    ) {
                        answers[question.id] = option.label
                    }
                }
            }
        }
    }

    private var answerBinding: Binding<String> {
        Binding(
            get: { answers[question.id] ?? "" },
            set: { answers[question.id] = $0 }
        )
    }
}

private struct OptionAnswerButton: View {
    let option: AskUserQuestion.Option
    let isSelected: Bool
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack {
                Image(systemName: isSelected ? "largecircle.fill.circle" : "circle")
                VStack(alignment: .leading) {
                    Text(option.label)
                    if let detail = option.description, !detail.isEmpty {
                        Text(detail)
                            .font(Typography.caption)
                            .foregroundStyle(Theme.foregroundTertiary)
                    }
                }
                Spacer()
            }
        }
        .buttonStyle(.plain)
    }
}
