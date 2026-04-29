using System;
using System.Diagnostics;
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

            var deployScript = Path.Combine(installDir, "deploy_mt5.ps1");
            if (File.Exists(deployScript))
            {
                RunProcess("powershell.exe", "-NoProfile -ExecutionPolicy Bypass -File \"" + deployScript + "\"", installDir, false);
            }

            CreateDesktopLaunchers(installDir);
            StartBridge(installDir);

            try
            {
                Process.Start(new ProcessStartInfo("http://127.0.0.1:8080/setup") { UseShellExecute = true });
            }
            catch
            {
            }

            MessageBox.Show(
                "Da cai xong DNSE MT5 Connector.\n\nHay mo trang setup de dien API key neu can.",
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
                "DNSE MT5 Connector Installer",
                MessageBoxButtons.OK,
                MessageBoxIcon.Error);
            return 1;
        }
    }

    private static void StartBridge(string installDir)
    {
        var bridgeExe = Path.Combine(installDir, "bridge.exe");
        if (!File.Exists(bridgeExe))
        {
            return;
        }

        var pidPath = Path.Combine(installDir, "bridge.pid");
        if (File.Exists(pidPath))
        {
            try
            {
                var raw = File.ReadAllText(pidPath).Trim();
                int pid;
                if (int.TryParse(raw, out pid))
                {
                    var existing = Process.GetProcessById(pid);
                    if (!existing.HasExited)
                    {
                        return;
                    }
                }
            }
            catch
            {
            }
        }

        var process = Process.Start(new ProcessStartInfo
        {
            FileName = bridgeExe,
            WorkingDirectory = installDir,
            UseShellExecute = true,
            WindowStyle = ProcessWindowStyle.Hidden
        });

        if (process != null)
        {
            File.WriteAllText(pidPath, process.Id.ToString(), Encoding.ASCII);
        }
    }

    private static void CreateDesktopLaunchers(string installDir)
    {
        var desktop = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Connector.bat"), installDir, Path.Combine(installDir, "start_trial.bat"));
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Setup.bat"), installDir, Path.Combine(installDir, "open_setup.bat"));
        WriteCmdLauncher(Path.Combine(desktop, "DNSE MT5 Stop.bat"), installDir, Path.Combine(installDir, "stop_trial.bat"));
    }

    private static void WriteCmdLauncher(string path, string installDir, string target)
    {
        var content = "@echo off\r\ncd /d \"" + installDir + "\"\r\nstart \"\" \"" + target + "\"\r\n";
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
}
