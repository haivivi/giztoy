package com.example.helloworld.kotlin

import android.app.Activity
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.view.animation.Animation
import android.view.animation.ScaleAnimation
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView

/**
 * Main activity demonstrating a Kotlin Android app built with Bazel.
 */
class MainActivity : Activity() {

    private lateinit var titleText: TextView
    private lateinit var subtitleText: TextView
    private lateinit var actionButton: Button
    private var tapCount = 0

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(createLayout())
    }

    /**
     * Creates the UI layout programmatically using Kotlin DSL-style.
     */
    private fun createLayout(): View {
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER
            setBackgroundColor(0xFFF5F5F5.toInt())
            setPadding(48, 48, 48, 48)

            // Title
            titleText = TextView(this@MainActivity).apply {
                text = "Hello, Bazel!"
                textSize = 32f
                setTextColor(0xFF212121.toInt())
                gravity = Gravity.CENTER
                setPadding(0, 0, 0, 16)
            }
            addView(titleText)

            // Subtitle
            subtitleText = TextView(this@MainActivity).apply {
                text = "Built with rules_kotlin 🎯"
                textSize = 18f
                setTextColor(0xFF757575.toInt())
                gravity = Gravity.CENTER
                setPadding(0, 0, 0, 48)
            }
            addView(subtitleText)

            // Button
            actionButton = Button(this@MainActivity).apply {
                text = "Tap Me!"
                textSize = 18f
                setTextColor(0xFFFFFFFF.toInt())
                setBackgroundColor(0xFF9C27B0.toInt())  // Material Purple
                setPadding(64, 24, 64, 24)
                setOnClickListener { onButtonClick() }
            }
            addView(actionButton)

            // Info section
            addView(LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER
                setPadding(0, 64, 0, 0)

                listOf(
                    "🔨 Bazel Build System",
                    "🎯 rules_kotlin",
                    "💜 Kotlin"
                ).forEach { line ->
                    addView(TextView(this@MainActivity).apply {
                        text = line
                        textSize = 14f
                        setTextColor(0xFF9E9E9E.toInt())
                        gravity = Gravity.CENTER
                        setPadding(0, 8, 0, 8)
                    })
                }
            })
        }
    }

    /**
     * Handles button click with animation.
     */
    private fun onButtonClick() {
        tapCount++

        // Update title
        val countText = if (tapCount == 1) "time" else "times"
        titleText.text = "Tapped $tapCount $countText!"

        // Animate button
        val scaleDown = ScaleAnimation(
            1.0f, 0.95f, 1.0f, 0.95f,
            Animation.RELATIVE_TO_SELF, 0.5f,
            Animation.RELATIVE_TO_SELF, 0.5f
        ).apply {
            duration = 100
            fillAfter = false
            setAnimationListener(object : Animation.AnimationListener {
                override fun onAnimationStart(animation: Animation?) {}
                override fun onAnimationRepeat(animation: Animation?) {}
                override fun onAnimationEnd(animation: Animation?) {
                    val scaleUp = ScaleAnimation(
                        0.95f, 1.0f, 0.95f, 1.0f,
                        Animation.RELATIVE_TO_SELF, 0.5f,
                        Animation.RELATIVE_TO_SELF, 0.5f
                    ).apply { duration = 100 }
                    actionButton.startAnimation(scaleUp)
                }
            })
        }
        actionButton.startAnimation(scaleDown)
    }
}
