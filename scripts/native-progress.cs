using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;

public static class ProgressWindows
{
    public sealed class Window
    {
        public IntPtr Handle;
        public int PID;
        public string Title;
    }
    private delegate bool EnumCallback(IntPtr handle, IntPtr parameter);
    [DllImport("user32.dll")]
    private static extern bool EnumWindows(EnumCallback callback, IntPtr parameter);
    [DllImport("user32.dll")]
    private static extern bool IsWindowVisible(IntPtr handle);
    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr handle, out int pid);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int GetWindowText(IntPtr handle, StringBuilder text, int count);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern IntPtr SendMessageTimeout(IntPtr window, uint message, IntPtr wparam, IntPtr lparam, uint flags, uint timeout, out IntPtr result);
    [DllImport("user32.dll", EntryPoint = "GetWindowLongW")]
    private static extern int GetWindowLong(IntPtr window, int index);

    public static Window[] All()
    {
        var windows = new List<Window>();
        EnumWindows((handle, parameter) => {
            if (!IsWindowVisible(handle)) return true;
            int pid;
            GetWindowThreadProcessId(handle, out pid);
            var title = new StringBuilder(1024);
            GetWindowText(handle, title, title.Capacity);
            windows.Add(new Window { Handle = handle, PID = pid, Title = title.ToString() });
            return true;
        }, IntPtr.Zero);
        return windows.ToArray();
    }

    private static int Read(IntPtr window, uint message, int parameter)
    {
        IntPtr result;
        if (SendMessageTimeout(window, message, new IntPtr(parameter), IntPtr.Zero, 2, 250, out result) == IntPtr.Zero) throw new InvalidOperationException("Progress control timed out");
        return unchecked((int)result.ToInt64());
    }

    public static double Value(IntPtr window)
    {
        if ((GetWindowLong(window, -16) & 8) != 0) throw new InvalidOperationException("Indeterminate progress");
        if (Read(window, 0x0411, 0) != 1) throw new InvalidOperationException("Progress is paused or in error state");
        int minimum = Read(window, 0x0407, 1);
        int maximum = Read(window, 0x0407, 0);
        int value = Read(window, 0x0408, 0);
        if (maximum <= minimum || value < minimum || value > maximum) throw new InvalidOperationException("Invalid progress range");
        return 100.0 * (value - (double)minimum) / (maximum - (double)minimum);
    }
}
