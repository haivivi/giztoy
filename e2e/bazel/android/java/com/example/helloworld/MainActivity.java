package com.example.helloworld;

import android.app.Activity;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.view.animation.Animation;
import android.view.animation.ScaleAnimation;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

/**
 * Main activity demonstrating a simple Android app built with Bazel.
 */
public class MainActivity extends Activity {

    private TextView titleText;
    private TextView subtitleText;
    private Button actionButton;
    private int tapCount = 0;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(createLayout());
    }

    /**
     * Creates the UI layout programmatically.
     */
    private View createLayout() {
        // Root container
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setGravity(Gravity.CENTER);
        root.setBackgroundColor(0xFFF5F5F5);  // Light gray background
        root.setPadding(48, 48, 48, 48);

        // Title
        titleText = new TextView(this);
        titleText.setText("Hello, Bazel!");
        titleText.setTextSize(32);
        titleText.setTextColor(0xFF212121);  // Dark gray
        titleText.setGravity(Gravity.CENTER);
        titleText.setPadding(0, 0, 0, 16);

        // Subtitle
        subtitleText = new TextView(this);
        subtitleText.setText("Built with rules_android 🤖");
        subtitleText.setTextSize(18);
        subtitleText.setTextColor(0xFF757575);  // Medium gray
        subtitleText.setGravity(Gravity.CENTER);
        subtitleText.setPadding(0, 0, 0, 48);

        // Button
        actionButton = new Button(this);
        actionButton.setText("Tap Me!");
        actionButton.setTextSize(18);
        actionButton.setTextColor(0xFFFFFFFF);  // White
        actionButton.setBackgroundColor(0xFF2196F3);  // Material Blue
        actionButton.setPadding(64, 24, 64, 24);
        actionButton.setOnClickListener(v -> onButtonClick());

        // Add views to root
        root.addView(titleText);
        root.addView(subtitleText);
        root.addView(actionButton);

        // Info section
        LinearLayout infoSection = new LinearLayout(this);
        infoSection.setOrientation(LinearLayout.VERTICAL);
        infoSection.setGravity(Gravity.CENTER);
        infoSection.setPadding(0, 64, 0, 0);

        String[] infoLines = {
            "🔨 Bazel Build System",
            "🤖 rules_android",
            "☕ Pure Java"
        };

        for (String line : infoLines) {
            TextView infoText = new TextView(this);
            infoText.setText(line);
            infoText.setTextSize(14);
            infoText.setTextColor(0xFF9E9E9E);  // Light gray
            infoText.setGravity(Gravity.CENTER);
            infoText.setPadding(0, 8, 0, 8);
            infoSection.addView(infoText);
        }

        root.addView(infoSection);

        return root;
    }

    /**
     * Handles button click with animation.
     */
    private void onButtonClick() {
        tapCount++;

        // Update title
        String countText = tapCount == 1 ? "time" : "times";
        titleText.setText("Tapped " + tapCount + " " + countText + "!");

        // Animate button
        ScaleAnimation scaleDown = new ScaleAnimation(
            1.0f, 0.95f, 1.0f, 0.95f,
            Animation.RELATIVE_TO_SELF, 0.5f,
            Animation.RELATIVE_TO_SELF, 0.5f
        );
        scaleDown.setDuration(100);
        scaleDown.setFillAfter(false);

        scaleDown.setAnimationListener(new Animation.AnimationListener() {
            @Override
            public void onAnimationStart(Animation animation) {}

            @Override
            public void onAnimationEnd(Animation animation) {
                ScaleAnimation scaleUp = new ScaleAnimation(
                    0.95f, 1.0f, 0.95f, 1.0f,
                    Animation.RELATIVE_TO_SELF, 0.5f,
                    Animation.RELATIVE_TO_SELF, 0.5f
                );
                scaleUp.setDuration(100);
                actionButton.startAnimation(scaleUp);
            }

            @Override
            public void onAnimationRepeat(Animation animation) {}
        });

        actionButton.startAnimation(scaleDown);
    }
}
