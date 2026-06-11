package lavender.client.android.ui.remote

import android.app.AlertDialog
import android.content.Context
import android.view.LayoutInflater
import android.widget.CheckBox
import android.widget.Toast
import com.google.android.material.textfield.TextInputEditText
import lavender.client.android.R
import lavender.client.android.theme.Theme
import lavender.client.android.theme.ThemeStore
import lavender.client.android.theme.ThemeUtils

/**
 * Dialog for generating an agent token.
 * Collects: Agent Name, Capabilities (multi-select), TTL.
 */
class TokenDialog(
    private val context: Context,
    private val theme: Theme = ThemeStore.currentTheme(),
    private val onGenerate: (agentName: String, capabilities: List<String>, ttlHours: Int) -> Unit
) {
    private var dialog: AlertDialog? = null

    fun show() {
        val view = LayoutInflater.from(context).inflate(R.layout.dialog_token_generate, null)

        val editAgentName = view.findViewById<TextInputEditText>(R.id.editAgentName)
        val editTtl = view.findViewById<TextInputEditText>(R.id.editTtl)

        val capShell = view.findViewById<CheckBox>(R.id.capShell)
        val capFile = view.findViewById<CheckBox>(R.id.capFile)
        val capGit = view.findViewById<CheckBox>(R.id.capGit)
        val capBuild = view.findViewById<CheckBox>(R.id.capBuild)
        val capDeploy = view.findViewById<CheckBox>(R.id.capDeploy)
        val capDocker = view.findViewById<CheckBox>(R.id.capDocker)
        val capAI = view.findViewById<CheckBox>(R.id.capAI)
        val capCustom = view.findViewById<CheckBox>(R.id.capCustom)

        val bgColor = ThemeUtils.parseSafeColor(theme.surfaceColor)
        val txtColor = ThemeUtils.parseSafeColor(theme.textPrimaryColor)
        val primColor = ThemeUtils.parseSafeColor(theme.primaryColor)

        dialog = AlertDialog.Builder(context)
            .setView(view)
            .setPositiveButton("Сгенерировать") { _, _ ->
                val name = editAgentName.text?.toString()?.trim() ?: ""
                if (name.isEmpty()) {
                    Toast.makeText(context, "Введите имя агента", Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }

                val capabilities = mutableListOf<String>()
                if (capShell.isChecked) capabilities.add("shell")
                if (capFile.isChecked) capabilities.add("file")
                if (capGit.isChecked) capabilities.add("git")
                if (capBuild.isChecked) capabilities.add("build")
                if (capDeploy.isChecked) capabilities.add("deploy")
                if (capDocker.isChecked) capabilities.add("docker")
                if (capAI.isChecked) capabilities.add("ai")
                if (capCustom.isChecked) capabilities.add("custom")

                if (capabilities.isEmpty()) {
                    Toast.makeText(context, "Выберите хотя бы одну возможность", Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }

                val ttl = editTtl.text?.toString()?.toIntOrNull() ?: 24

                onGenerate(name, capabilities, ttl)
            }
            .setNegativeButton("Отмена", null)
            .create()

        dialog?.show()

        // Apply theme to dialog buttons
        dialog?.getButton(AlertDialog.BUTTON_POSITIVE)?.setTextColor(primColor)
        dialog?.getButton(AlertDialog.BUTTON_NEGATIVE)?.setTextColor(txtColor)
    }

    fun dismiss() {
        dialog?.dismiss()
        dialog = null
    }
}
