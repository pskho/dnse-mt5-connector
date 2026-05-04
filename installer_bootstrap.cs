using System;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.IO.Compression;
using System.Reflection;
using System.Text;
using System.Windows.Forms;

internal static class Program
{
    [STAThread]
    private static int Main()
    {
        try
        {
            var installDir = Path.Combine(
                Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "DNSE-MT5-Connector");

            var tempRoot = Path.Combine(Path.GetTempPath(), "dnse-mt5-install-" + Guid.NewGuid().ToString("N"));
            Directory.CreateDirectory(tempRoot);

            var payloadZip = Path.Combine(tempRoot, "payload.zip");
            using (var resource = Assembly.GetExecutingAssembly().GetManifestResourceStream("payload.zip"))
            {
                if (resource == null)
                {
                    throw new InvalidOperationException("Khong tim thay payload.zip trong installer.");
                }

                using (var output = File.Create(payloadZip))
                {
                    resource.CopyTo(output);
                }
            }

            var extractDir = Path.Combine(tempRoot, "payload");
            ZipFile.ExtractToDirectory(payloadZip, extractDir);

            Directory.CreateDirectory(installDir);
            CopyDirectorySafe(extractDir, installDir);

            var configExample = Path.Combine(installDir, "config", "config.yaml.example");
            var configPath = Path.Combine(installDir, "config", "config.yaml");
            if (!File.Exists(configPath) && File.Exists(configExample))
            {
                File.Copy(configExample, configPath, true);
            }

            EnsureInitialConfig(configPath);

            var deployScript = Path.Combine(installDir, "deploy_mt5.ps1");
            if (File.Exists(deployScript))
            {
                RunProcess("powershell.exe", "-NoProfile -ExecutionPolicy Bypass -File \"" + deployScript + "\"", installDir, false);
            }

            CreateDesktopLaunchers(installDir);
            StartTrial(installDir);

            try
            {
                Process.Start(new ProcessStartInfo("explorer.exe", "\"" + installDir + "\"") { UseShellExecute = true });
            }
            catch
            {
            }

            try
            {
                Process.Start(new ProcessStartInfo("http://127.0.0.1:8080/setup") { UseShellExecute = true });
            }
            catch
            {
            }

            MessageBox.Show(
                "Đã cài xong DNSE MT5 Connector.\n\nThư mục cài đặt:\n" + installDir +
                "\n\nTrình cài đặt đã mở sẵn thư mục này cho bạn." +
                "\nNếu bridge chưa chạy, hãy bấm file DNSE MT5 Connector.bat trong thư mục này." +
                "\nNếu vẫn chưa rõ lỗi, hãy bấm run_bridge_console.bat để xem lỗi trực tiếp." +
                "\nNếu Windows hiện cảnh báo khi chạy bridge, hãy chọn \"More info\" rồi \"Run anyway\".",
                "DNSE MT5 Connector",
                MessageBoxButtons.OK,
                MessageBoxIcon.Information);

            TryDeleteDirectory(tempRoot);
            return 0;
        }
        catch (Exception ex)
        {
            MessageBox.Show(
                ex.ToString(),
                "Trình cài đặt DNSE MT5 Connector",
                MessageBoxButtons.OK,
                MessageBoxIcon.Error);
            return 1;
        }
    }

    private static void StartTrial(string installDir)
    {
        var startScript = Path.Combine(installDir, "start_trial.bat");
        if (!File.Exists(startScript))
        {
            return;
        }

        Process.Start(new ProcessStartInfo
        {
            FileName = "cmd.exe",
            Arguments = "/c \"\"" + startScript + "\"\"",
            WorkingDirectory = installDir,
            UseShellExecute = true,
            WindowStyle = ProcessWindowStyle.Normal
        });
    }

    private static void EnsureInitialConfig(string configPath)
    {
        if (!File.Exists(configPath))
        {
            return;
        }

        var content = File.ReadAllText(configPath, Encoding.UTF8);
        var needsSetup =
            content.Contains("PASTE_DNSE_API_KEY_HERE") ||
            content.Contains("PASTE_DNSE_API_SECRET_HERE") ||
            content.Contains("PASTE_ACCOUNT_NO_HERE");

        if (!needsSetup)
        {
            return;
        }

        using (var form = new FirstRunConfigForm())
        {
            var result = form.ShowDialog();
            if (result != DialogResult.OK)
            {
                throw new InvalidOperationException("Bạn chưa nhập thông tin DNSE nên không thể tiếp tục cài đặt.");
            }

            content = content
                .Replace("PASTE_DNSE_API_KEY_HERE", form.ApiKey.Trim())
                .Replace("PASTE_DNSE_API_SECRET_HERE", form.ApiSecret.Trim())
                .Replace("PASTE_ACCOUNT_NO_HERE", form.AccountNo.Trim());

            File.WriteAllText(configPath, content, Encoding.UTF8);
        }
    }

    private static void CreateDesktopLaunchers(string installDir)
    {
        var desktop = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Connector.bat"), installDir, Path.Combine(installDir, "start_trial.bat"));
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Setup.bat"), installDir, Path.Combine(installDir, "open_setup.bat"));
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Stop.bat"), installDir, Path.Combine(installDir, "stop_trial.bat"));
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Folder.bat"), installDir, "explorer.exe \"" + installDir + "\"");
    }

    private static void WriteCmdLauncher(string path, string installDir, string target)
    {
        var content = "@echo off\r\ncd /d \"" + installDir + "\"\r\n" +
                      (target.StartsWith("explorer.exe ", StringComparison.OrdinalIgnoreCase)
                          ? target + "\r\n"
                          : "start \"\" \"" + target + "\"\r\n");
        File.WriteAllText(path, content, Encoding.ASCII);
    }

    private static void RunProcess(string fileName, string arguments, string workingDirectory, bool hidden)
    {
        using (var process = Process.Start(new ProcessStartInfo
        {
            FileName = fileName,
            Arguments = arguments,
            WorkingDirectory = workingDirectory,
            UseShellExecute = false,
            CreateNoWindow = hidden,
            WindowStyle = hidden ? ProcessWindowStyle.Hidden : ProcessWindowStyle.Normal
        }))
        {
            if (process != null)
            {
                process.WaitForExit();
            }
        }
    }

    private static void CopyDirectorySafe(string sourceDir, string destinationDir)
    {
        foreach (var directory in Directory.GetDirectories(sourceDir, "*", SearchOption.AllDirectories))
        {
            var relative = directory.Substring(sourceDir.Length).TrimStart(Path.DirectorySeparatorChar);
            Directory.CreateDirectory(Path.Combine(destinationDir, relative));
        }

        foreach (var file in Directory.GetFiles(sourceDir, "*", SearchOption.AllDirectories))
        {
            var relative = file.Substring(sourceDir.Length).TrimStart(Path.DirectorySeparatorChar);
            var target = Path.Combine(destinationDir, relative);
            var parent = Path.GetDirectoryName(target);
            if (!string.IsNullOrEmpty(parent))
            {
                Directory.CreateDirectory(parent);
            }

            if (string.Equals(relative, Path.Combine("config", "config.yaml"), StringComparison.OrdinalIgnoreCase) && File.Exists(target))
            {
                continue;
            }

            File.Copy(file, target, true);
        }
    }

    private static void TryDeleteDirectory(string path)
    {
        try
        {
            if (Directory.Exists(path))
            {
                Directory.Delete(path, true);
            }
        }
        catch
        {
        }
    }

    private sealed class FirstRunConfigForm : Form
    {
        private readonly TextBox apiKeyBox = new TextBox();
        private readonly TextBox apiSecretBox = new TextBox();
        private readonly TextBox accountNoBox = new TextBox();

        public string ApiKey
        {
            get { return apiKeyBox.Text; }
        }

        public string ApiSecret
        {
            get { return apiSecretBox.Text; }
        }

        public string AccountNo
        {
            get { return accountNoBox.Text; }
        }

        public FirstRunConfigForm()
        {
            Text = "DNSE MT5 Connector";
            Width = 640;
            Height = 420;
            FormBorderStyle = FormBorderStyle.FixedDialog;
            MaximizeBox = false;
            MinimizeBox = false;
            StartPosition = FormStartPosition.CenterScreen;
            Font = new Font("Segoe UI", 9F, FontStyle.Regular, GraphicsUnit.Point);
            BackColor = Color.FromArgb(245, 247, 250);

            var brand = new Label
            {
                Left = 28,
                Top = 22,
                Width = 260,
                Height = 28,
                Font = new Font("Segoe UI Semibold", 15F, FontStyle.Bold, GraphicsUnit.Point),
                ForeColor = Color.FromArgb(216, 45, 45),
                Text = "DNSE Connector"
            };

            var heading = new Label
            {
                Left = 30,
                Top = 58,
                Width = 300,
                Height = 24,
                Font = new Font("Segoe UI Semibold", 12F, FontStyle.Bold, GraphicsUnit.Point),
                ForeColor = Color.FromArgb(24, 28, 33),
                Text = "Thiết lập lần đầu"
            };

            var intro = new Label
            {
                Left = 30,
                Top = 88,
                Width = 560,
                Height = 40,
                ForeColor = Color.FromArgb(87, 96, 106),
                Text = "Nhập thông tin DNSE trước khi bridge khởi động. Sau bước này phần mềm sẽ kết nối và tự động chuẩn bị dữ liệu ban đầu cho MT5."
            };

            var card = new Panel
            {
                Left = 30,
                Top = 138,
                Width = 560,
                Height = 180,
                BackColor = Color.White,
                BorderStyle = BorderStyle.FixedSingle
            };

            var apiKeyLabel = new Label
            {
                Left = 22,
                Top = 22,
                Width = 120,
                Height = 22,
                ForeColor = Color.FromArgb(60, 66, 74),
                Text = "Khóa API"
            };
            apiKeyBox.Left = 22;
            apiKeyBox.Top = 46;
            apiKeyBox.Width = 514;
            apiKeyBox.Height = 30;
            apiKeyBox.BorderStyle = BorderStyle.FixedSingle;

            var apiSecretLabel = new Label
            {
                Left = 22,
                Top = 82,
                Width = 120,
                Height = 22,
                ForeColor = Color.FromArgb(60, 66, 74),
                Text = "Mã bí mật API"
            };
            apiSecretBox.Left = 22;
            apiSecretBox.Top = 106;
            apiSecretBox.Width = 514;
            apiSecretBox.Height = 30;
            apiSecretBox.BorderStyle = BorderStyle.FixedSingle;
            apiSecretBox.UseSystemPasswordChar = true;

            var accountNoLabel = new Label
            {
                Left = 22,
                Top = 142,
                Width = 120,
                Height = 22,
                ForeColor = Color.FromArgb(60, 66, 74),
                Text = "Số tài khoản"
            };
            accountNoBox.Left = 150;
            accountNoBox.Top = 152;
            accountNoBox.Width = 200;
            accountNoBox.Visible = false;

            var accountNoInline = new TextBox
            {
                Left = 22,
                Top = 166,
                Width = 240,
                Height = 30,
                BorderStyle = BorderStyle.FixedSingle
            };
            accountNoInline.DataBindings.Add("Text", accountNoBox, "Text", false, DataSourceUpdateMode.OnPropertyChanged);

            var okButton = new Button
            {
                Text = "Lưu và tiếp tục",
                Left = 350,
                Top = 338,
                Width = 118,
                Height = 34,
                FlatStyle = FlatStyle.Flat,
                BackColor = Color.FromArgb(216, 45, 45),
                ForeColor = Color.White,
                DialogResult = DialogResult.OK
            };
            okButton.FlatAppearance.BorderSize = 0;
            okButton.Click += (sender, args) =>
            {
                if (string.IsNullOrWhiteSpace(apiKeyBox.Text) ||
                    string.IsNullOrWhiteSpace(apiSecretBox.Text) ||
                    string.IsNullOrWhiteSpace(accountNoInline.Text))
                {
                    MessageBox.Show(
                        "Vui lòng nhập đầy đủ Khóa API, Mã bí mật API và Số tài khoản.",
                        "Thiếu thông tin",
                        MessageBoxButtons.OK,
                        MessageBoxIcon.Warning);
                    DialogResult = DialogResult.None;
                }
            };

            var cancelButton = new Button
            {
                Text = "Hủy",
                Left = 480,
                Top = 338,
                Width = 110,
                Height = 34,
                FlatStyle = FlatStyle.Flat,
                BackColor = Color.White,
                ForeColor = Color.FromArgb(60, 66, 74),
                DialogResult = DialogResult.Cancel
            };
            cancelButton.FlatAppearance.BorderColor = Color.FromArgb(210, 214, 220);

            var note = new Label
            {
                Left = 30,
                Top = 330,
                Width = 290,
                Height = 42,
                ForeColor = Color.FromArgb(110, 118, 129),
                Text = "Bạn chỉ cần nhập một lần. Từ các lần sau phần mềm sẽ dùng lại cấu hình đã lưu."
            };

            card.Controls.Add(apiKeyLabel);
            card.Controls.Add(apiKeyBox);
            card.Controls.Add(apiSecretLabel);
            card.Controls.Add(apiSecretBox);
            card.Controls.Add(accountNoLabel);
            card.Controls.Add(accountNoInline);

            Controls.Add(brand);
            Controls.Add(heading);
            Controls.Add(intro);
            Controls.Add(card);
            Controls.Add(note);
            Controls.Add(okButton);
            Controls.Add(cancelButton);
            Controls.Add(accountNoBox);

            AcceptButton = okButton;
            CancelButton = cancelButton;
        }
    }
}
