import UIKit
import Causality

class ViewController: UIViewController {

    private let stackView: UIStackView = {
        let stack = UIStackView()
        stack.axis = .vertical
        stack.spacing = 16
        stack.translatesAutoresizingMaskIntoConstraints = false
        return stack
    }()

    private let deviceIdLabel: UILabel = {
        let label = UILabel()
        label.textAlignment = .center
        label.numberOfLines = 0
        label.font = .systemFont(ofSize: 12, weight: .medium)
        label.textColor = .secondaryLabel
        return label
    }()

    override func viewDidLoad() {
        super.viewDidLoad()

        title = "Causality UIKit Example"
        view.backgroundColor = .systemBackground

        setupUI()
        trackScreenView()
        updateDeviceIdLabel()
    }

    // MARK: - UI Setup

    private func setupUI() {
        view.addSubview(stackView)
        NSLayoutConstraint.activate([
            stackView.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            stackView.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            stackView.leadingAnchor.constraint(equalTo: view.leadingAnchor, constant: 20),
            stackView.trailingAnchor.constraint(equalTo: view.trailingAnchor, constant: -20)
        ])

        // Device ID label
        stackView.addArrangedSubview(deviceIdLabel)

        // Track Button
        let trackButton = createButton(
            title: "Track Event",
            action: #selector(trackButtonTapped)
        )
        stackView.addArrangedSubview(trackButton)

        // Identify Button
        let identifyButton = createButton(
            title: "Identify User",
            action: #selector(identifyTapped)
        )
        stackView.addArrangedSubview(identifyButton)

        // Flush Button
        let flushButton = createButton(
            title: "Flush Events",
            action: #selector(flushTapped)
        )
        stackView.addArrangedSubview(flushButton)

        // Reset Button
        let resetButton = createButton(
            title: "Reset User",
            action: #selector(resetTapped)
        )
        resetButton.backgroundColor = .systemOrange
        stackView.addArrangedSubview(resetButton)
    }

    private func createButton(title: String, action: Selector) -> UIButton {
        let button = UIButton(type: .system)
        button.setTitle(title, for: .normal)
        button.backgroundColor = .systemBlue
        button.setTitleColor(.white, for: .normal)
        button.layer.cornerRadius = 8
        button.heightAnchor.constraint(equalToConstant: 44).isActive = true
        button.addTarget(self, action: action, for: .touchUpInside)
        return button
    }

    private func updateDeviceIdLabel() {
        deviceIdLabel.text = "Device ID: \(Causality.shared.deviceId)"
    }

    // MARK: - SDK Integration

    private func trackScreenView() {
        Causality.shared.track(ScreenView(screenName: "main_screen"))
    }

    @objc private func trackButtonTapped() {
        Causality.shared.track(ButtonTap(buttonId: "track_event", screenName: "main_screen"))
        print("[UIKitExample] Event tracked: button_tap")

        // Visual feedback
        showToast(message: "Event tracked!")
    }

    @objc private func identifyTapped() {
        do {
            try Causality.shared.identify(
                userId: "user-123",
                traits: [
                    "name": AnyCodable("John Doe"),
                    "email": AnyCodable("john@example.com"),
                    "plan": AnyCodable("premium")
                ]
            )
            print("[UIKitExample] User identified: user-123")
            showToast(message: "User identified!")
        } catch {
            print("[UIKitExample] Identify error: \(error)")
            showToast(message: "Error: \(error.localizedDescription)")
        }
    }

    @objc private func flushTapped() {
        Task {
            do {
                try await Causality.shared.flush()
                print("[UIKitExample] Events flushed")
                await MainActor.run {
                    showToast(message: "Events flushed!")
                }
            } catch {
                print("[UIKitExample] Flush error: \(error)")
                await MainActor.run {
                    showToast(message: "Error: \(error.localizedDescription)")
                }
            }
        }
    }

    @objc private func resetTapped() {
        do {
            try Causality.shared.reset()
            print("[UIKitExample] User reset")
            showToast(message: "User reset!")
        } catch {
            print("[UIKitExample] Reset error: \(error)")
            showToast(message: "Error: \(error.localizedDescription)")
        }
    }

    // MARK: - Helpers

    private func showToast(message: String) {
        let alert = UIAlertController(title: nil, message: message, preferredStyle: .alert)
        present(alert, animated: true)
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5) {
            alert.dismiss(animated: true)
        }
    }
}
