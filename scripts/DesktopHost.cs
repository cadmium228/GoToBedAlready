using System;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Net;
using System.Runtime.InteropServices;
using System.Windows.Forms;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.WinForms;

namespace GoToBedAlready
{
    public static class DesktopHost
    {
        [DllImport("dwmapi.dll")]
        private static extern int DwmSetWindowAttribute(IntPtr window, int attribute, ref int value, int size);

        [DllImport("shell32.dll", CharSet = CharSet.Unicode)]
        private static extern int SetCurrentProcessExplicitAppUserModelID(string applicationId);

        public static void Run(string address)
        {
            SetCurrentProcessExplicitAppUserModelID("GoToBedAlready.Desktop");
            Application.EnableVisualStyles();
            Application.SetCompatibleTextRenderingDefault(false);
            var location = new Uri(address);
            var window = new Form {
                Text = "GoToBedAlready",
                ClientSize = new Size(560, 620),
                MinimumSize = new Size(440, 580),
                StartPosition = FormStartPosition.CenterScreen,
                BackColor = Color.FromArgb(30, 32, 35),
                AutoScaleMode = AutoScaleMode.Dpi
            };
            var browser = new WebView2 {
                Dock = DockStyle.Fill,
                DefaultBackgroundColor = window.BackColor,
                CreationProperties = new CoreWebView2CreationProperties {
                    UserDataFolder = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "GoToBedAlready", "WebView2")
                }
            };
            window.Controls.Add(browser);
            var parent = Process.GetProcessById(int.Parse(Environment.GetEnvironmentVariable("GTB_PARENT")));
            var parentWatch = new Timer { Interval = 1000 };
            parentWatch.Tick += (sender, args) => { if (parent.HasExited) window.Close(); };
            parentWatch.Start();
            window.Shown += async (sender, args) => {
                try {
                    int dark = 1;
                    DwmSetWindowAttribute(window.Handle, 20, ref dark, sizeof(int));
                    await browser.EnsureCoreWebView2Async(null);
                    if (window.IsDisposed) return;
                    browser.CoreWebView2.Settings.AreDevToolsEnabled = false;
                    browser.CoreWebView2.Settings.AreDefaultContextMenusEnabled = false;
                    browser.CoreWebView2.Settings.IsStatusBarEnabled = false;
                    browser.CoreWebView2.Settings.AreHostObjectsAllowed = false;
                    browser.CoreWebView2.Settings.IsWebMessageEnabled = false;
                    browser.CoreWebView2.NewWindowRequested += (source, request) => { request.Handled = true; };
                    browser.CoreWebView2.PermissionRequested += (source, request) => { request.State = CoreWebView2PermissionState.Deny; };
                    browser.CoreWebView2.NavigationStarting += (source, request) => {
                        Uri destination;
                        if (!Uri.TryCreate(request.Uri, UriKind.Absolute, out destination) || destination.GetLeftPart(UriPartial.Authority) != location.GetLeftPart(UriPartial.Authority)) request.Cancel = true;
                    };
                    browser.CoreWebView2.Navigate(address);
                } catch (Exception error) {
                    MessageBox.Show(window, "WebView2: " + error.Message + "\nInstall Microsoft Edge WebView2 Runtime.", "GoToBedAlready", MessageBoxButtons.OK, MessageBoxIcon.Error);
                    window.Close();
                }
            };
            window.FormClosing += (sender, args) => {
                try {
                    var request = (HttpWebRequest)WebRequest.Create(new Uri(location, "/api/cancel"));
                    request.Method = "POST";
                    request.Headers["X-Session-Token"] = location.Fragment.TrimStart('#');
                    request.ContentLength = 0;
                    request.Timeout = 2000;
                    using (request.GetResponse()) {}
                } catch (WebException) {}
            };
            Application.Run(window);
            parentWatch.Dispose();
            parent.Dispose();
            browser.Dispose();
            window.Dispose();
        }
    }
}
