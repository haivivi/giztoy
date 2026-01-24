import SwiftUI

struct ContentView: View {
    @State private var tapCount = 0
    @State private var isAnimating = false

    var body: some View {
        ZStack {
            // Gradient background
            LinearGradient(
                gradient: Gradient(colors: [
                    Color.blue.opacity(0.6),
                    Color.purple.opacity(0.8)
                ]),
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            VStack(spacing: 30) {
                // Logo/Icon
                Image(systemName: "hammer.fill")
                    .font(.system(size: 80))
                    .foregroundColor(.white)
                    .rotationEffect(.degrees(isAnimating ? 10 : -10))
                    .animation(
                        Animation.easeInOut(duration: 0.5)
                            .repeatForever(autoreverses: true),
                        value: isAnimating
                    )

                // Title
                Text("Hello, Bazel!")
                    .font(.system(size: 36, weight: .bold, design: .rounded))
                    .foregroundColor(.white)

                // Subtitle
                Text("Built with rules_apple + SwiftUI")
                    .font(.subheadline)
                    .foregroundColor(.white.opacity(0.8))

                // Tap counter
                if tapCount > 0 {
                    Text("Tapped \(tapCount) time\(tapCount == 1 ? "" : "s")")
                        .font(.title2)
                        .foregroundColor(.white)
                        .transition(.scale.combined(with: .opacity))
                }

                // Button
                Button(action: {
                    withAnimation(.spring()) {
                        tapCount += 1
                    }
                }) {
                    HStack {
                        Image(systemName: "hand.tap.fill")
                        Text("Tap Me!")
                    }
                    .font(.headline)
                    .foregroundColor(.blue)
                    .padding(.horizontal, 40)
                    .padding(.vertical, 16)
                    .background(Color.white)
                    .cornerRadius(25)
                    .shadow(color: .black.opacity(0.2), radius: 10, x: 0, y: 5)
                }

                Spacer()

                // Footer
                VStack(spacing: 8) {
                    Text("🔨 Bazel Build System")
                    Text("🍎 rules_apple")
                    Text("🦅 rules_swift")
                }
                .font(.caption)
                .foregroundColor(.white.opacity(0.7))
            }
            .padding(.top, 60)
            .padding(.bottom, 40)
        }
        .onAppear {
            isAnimating = true
        }
    }
}

#Preview {
    ContentView()
}
