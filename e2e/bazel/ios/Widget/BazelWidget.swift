import WidgetKit
import SwiftUI

// MARK: - Timeline Entry

struct BazelWidgetEntry: TimelineEntry {
    let date: Date
    let buildCount: Int
    let lastBuildTime: String
    let status: BuildStatus
}

enum BuildStatus: String {
    case success = "Success"
    case failed = "Failed"
    case building = "Building"
    
    var color: Color {
        switch self {
        case .success: return .green
        case .failed: return .red
        case .building: return .orange
        }
    }
    
    var icon: String {
        switch self {
        case .success: return "checkmark.circle.fill"
        case .failed: return "xmark.circle.fill"
        case .building: return "arrow.triangle.2.circlepath"
        }
    }
}

// MARK: - Timeline Provider

struct BazelWidgetProvider: TimelineProvider {
    func placeholder(in context: Context) -> BazelWidgetEntry {
        BazelWidgetEntry(
            date: Date(),
            buildCount: 42,
            lastBuildTime: "Just now",
            status: .success
        )
    }

    func getSnapshot(in context: Context, completion: @escaping (BazelWidgetEntry) -> Void) {
        let entry = BazelWidgetEntry(
            date: Date(),
            buildCount: 128,
            lastBuildTime: "2 min ago",
            status: .success
        )
        completion(entry)
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<BazelWidgetEntry>) -> Void) {
        var entries: [BazelWidgetEntry] = []
        let currentDate = Date()
        
        // Generate entries for the next 5 hours
        for hourOffset in 0..<5 {
            let entryDate = Calendar.current.date(byAdding: .hour, value: hourOffset, to: currentDate)!
            let entry = BazelWidgetEntry(
                date: entryDate,
                buildCount: 128 + hourOffset,
                lastBuildTime: hourOffset == 0 ? "Just now" : "\(hourOffset)h ago",
                status: .success
            )
            entries.append(entry)
        }

        let timeline = Timeline(entries: entries, policy: .atEnd)
        completion(timeline)
    }
}

// MARK: - Widget Views

struct BazelWidgetSmallView: View {
    var entry: BazelWidgetEntry

    var body: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Image(systemName: "hammer.fill")
                    .font(.title2)
                    .foregroundColor(.blue)
                Spacer()
                Image(systemName: entry.status.icon)
                    .foregroundColor(entry.status.color)
            }
            
            Spacer()
            
            Text("Bazel")
                .font(.headline)
                .fontWeight(.bold)
            
            Text("\(entry.buildCount) builds")
                .font(.caption)
                .foregroundColor(.secondary)
        }
        .padding()
    }
}

struct BazelWidgetMediumView: View {
    var entry: BazelWidgetEntry

    var body: some View {
        HStack(spacing: 16) {
            // Left side - Icon and status
            VStack(alignment: .center, spacing: 8) {
                ZStack {
                    Circle()
                        .fill(Color.blue.opacity(0.2))
                        .frame(width: 60, height: 60)
                    Image(systemName: "hammer.fill")
                        .font(.system(size: 28))
                        .foregroundColor(.blue)
                }
                
                HStack(spacing: 4) {
                    Image(systemName: entry.status.icon)
                        .font(.caption)
                    Text(entry.status.rawValue)
                        .font(.caption)
                        .fontWeight(.medium)
                }
                .foregroundColor(entry.status.color)
            }
            
            // Right side - Stats
            VStack(alignment: .leading, spacing: 6) {
                Text("Bazel Build")
                    .font(.headline)
                    .fontWeight(.bold)
                
                Divider()
                
                HStack {
                    VStack(alignment: .leading) {
                        Text("Total Builds")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                        Text("\(entry.buildCount)")
                            .font(.title3)
                            .fontWeight(.semibold)
                    }
                    
                    Spacer()
                    
                    VStack(alignment: .trailing) {
                        Text("Last Build")
                            .font(.caption2)
                            .foregroundColor(.secondary)
                        Text(entry.lastBuildTime)
                            .font(.caption)
                            .fontWeight(.medium)
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding()
    }
}

struct BazelWidgetLargeView: View {
    var entry: BazelWidgetEntry
    
    let recentBuilds = [
        ("//ios:SwiftUIApp", "Success", Color.green),
        ("//android:app", "Success", Color.green),
        ("//go/cmd/...", "Success", Color.green),
        ("//rust/...", "Building", Color.orange),
    ]

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            HStack {
                Image(systemName: "hammer.fill")
                    .font(.title2)
                    .foregroundColor(.blue)
                Text("Bazel Dashboard")
                    .font(.headline)
                    .fontWeight(.bold)
                Spacer()
                Image(systemName: entry.status.icon)
                    .foregroundColor(entry.status.color)
            }
            
            Divider()
            
            // Stats row
            HStack(spacing: 20) {
                StatBox(title: "Builds", value: "\(entry.buildCount)", icon: "number")
                StatBox(title: "Success", value: "98%", icon: "checkmark")
                StatBox(title: "Cache", value: "85%", icon: "externaldrive")
            }
            
            Divider()
            
            // Recent builds
            Text("Recent Builds")
                .font(.subheadline)
                .fontWeight(.semibold)
            
            ForEach(recentBuilds, id: \.0) { target, status, color in
                HStack {
                    Circle()
                        .fill(color)
                        .frame(width: 8, height: 8)
                    Text(target)
                        .font(.caption)
                        .lineLimit(1)
                    Spacer()
                    Text(status)
                        .font(.caption2)
                        .foregroundColor(.secondary)
                }
            }
            
            Spacer()
        }
        .padding()
    }
}

struct StatBox: View {
    let title: String
    let value: String
    let icon: String
    
    var body: some View {
        VStack(spacing: 4) {
            Image(systemName: icon)
                .font(.caption)
                .foregroundColor(.blue)
            Text(value)
                .font(.subheadline)
                .fontWeight(.bold)
            Text(title)
                .font(.caption2)
                .foregroundColor(.secondary)
        }
        .frame(maxWidth: .infinity)
    }
}

// MARK: - Widget Configuration

struct BazelWidget: Widget {
    let kind: String = "BazelWidget"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: BazelWidgetProvider()) { entry in
            BazelWidgetEntryView(entry: entry)
                .containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("Bazel Build")
        .description("Monitor your Bazel build status.")
        .supportedFamilies([.systemSmall, .systemMedium, .systemLarge])
    }
}

struct BazelWidgetEntryView: View {
    @Environment(\.widgetFamily) var family
    var entry: BazelWidgetProvider.Entry

    var body: some View {
        switch family {
        case .systemSmall:
            BazelWidgetSmallView(entry: entry)
        case .systemMedium:
            BazelWidgetMediumView(entry: entry)
        case .systemLarge:
            BazelWidgetLargeView(entry: entry)
        default:
            BazelWidgetSmallView(entry: entry)
        }
    }
}

// MARK: - Widget Bundle

@main
struct BazelWidgetBundle: WidgetBundle {
    var body: some Widget {
        BazelWidget()
    }
}

// MARK: - Previews

#Preview("Small", as: .systemSmall) {
    BazelWidget()
} timeline: {
    BazelWidgetEntry(date: .now, buildCount: 42, lastBuildTime: "Just now", status: .success)
    BazelWidgetEntry(date: .now, buildCount: 43, lastBuildTime: "1 min ago", status: .building)
}

#Preview("Medium", as: .systemMedium) {
    BazelWidget()
} timeline: {
    BazelWidgetEntry(date: .now, buildCount: 128, lastBuildTime: "2 min ago", status: .success)
}

#Preview("Large", as: .systemLarge) {
    BazelWidget()
} timeline: {
    BazelWidgetEntry(date: .now, buildCount: 256, lastBuildTime: "Just now", status: .success)
}
