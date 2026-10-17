# WebView2 SDK

Microsoft.Web.WebView2 **1.0.3856.49**: managed Core / WinForms assemblies and native WebView2Loader, unmodified.

License: `WebView2-LICENSE.txt`.
Official license: https://www.nuget.org/packages/Microsoft.Web.WebView2/1.0.3856.49/License

Build-time distribution source: the `pywebview 6.2.1` wheel on PyPI. SHA-256 of the complete wheel:
`9d07275f53894ab4d5e2e0e996227193e7187dec276d9b624dccbce029216b46`.
The build script verifies the digest before extracting the explicit DLL allowlist. Its MIT notice is also included as `pywebview-LICENSE.txt`. Python and pywebview are not runtime dependencies; no Python code is bundled into the application.

The Microsoft Edge WebView2 Runtime is separate from the SDK and must already be installed on Windows. It is not downloaded or installed automatically by the application.
