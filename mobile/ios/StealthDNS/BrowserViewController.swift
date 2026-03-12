import UIKit
import WebKit

class BrowserViewController: UIViewController {
    
    // MARK: - Tab Structure
    struct BrowserTab {
        let id: Int
        var title: String
        var url: String
        var isNhp: Bool
        var webViewData: Data?
    }
    
    // MARK: - UI Components
    private var webView: WKWebView!
    private var urlTextField: UITextField!
    private var progressView: UIProgressView!
    private var nhpIndicatorView: UIView!
    private var nhpStatusLabel: UILabel!
    private var securityImageView: UIImageView!
    private var navigationBar: UIStackView!
    private var backButton: UIButton!
    private var forwardButton: UIButton!
    private var refreshButton: UIButton!
    private var qrScanButton: UIButton!
    private var homeButton: UIButton!
    private var tabsButton: UIButton!
    private var menuButton: UIButton!
    
    // MARK: - Properties
    private var currentPageIsNhp = false
    private var progressObservation: NSKeyValueObservation?
    // Map: real URL (with resolved host) -> original NHP URL (with .nhp domain)
    // Used to restore NHP display URL when navigating back/forward
    private var nhpUrlMapping = [String: String]()
    // Current original NHP URL being displayed (to show in address bar instead of real URL)
    private var currentNhpDisplayUrl: String?
    
    // Tab management
    private var tabs: [BrowserTab] = []
    private var currentTabIndex = 0
    private var tabIdCounter = 0
    
    // MARK: - Colors
    private let backgroundColor = UIColor(red: 0.043, green: 0.063, blue: 0.125, alpha: 1.0)
    private let surfaceColor = UIColor(red: 0.082, green: 0.102, blue: 0.180, alpha: 1.0)
    private let nhpBlue = UIColor(red: 0.161, green: 0.714, blue: 0.965, alpha: 1.0)
    private let nhpGreen = UIColor(red: 0.298, green: 0.686, blue: 0.314, alpha: 1.0)
    private let nhpOrange = UIColor(red: 1.0, green: 0.596, blue: 0.0, alpha: 1.0)
    private let textPrimary = UIColor.white
    private let textSecondary = UIColor(white: 0.62, alpha: 1.0)
    
    // MARK: - Lifecycle
    override func viewDidLoad() {
        super.viewDidLoad()
        setupUI()
        setupWebView()
        setupObservers()
        
        // Create first tab
        createNewTab()
        
        // Load default URL after NHP initialization
        loadDefaultURLWhenReady()
    }
    
    deinit {
        progressObservation?.invalidate()
        NotificationCenter.default.removeObserver(self)
    }
    
    private func loadDefaultURLWhenReady() {
        // Check if NHP is already initialized
        if let appDelegate = UIApplication.shared.delegate as? AppDelegate, appDelegate.nhpInitialized {
            loadURL("https://demo.nhp")
            return
        }
        
        // Show loading indicator
        nhpIndicatorView.isHidden = false
        nhpStatusLabel.text = "Initializing NHP..."
        progressView.isHidden = false
        progressView.progress = 0.1
        
        // Wait for NHP initialization with timeout
        var attempts = 0
        let maxAttempts = 50 // 5 seconds timeout (50 * 100ms)
        
        func checkAndLoad() {
            if let appDelegate = UIApplication.shared.delegate as? AppDelegate, appDelegate.nhpInitialized {
                DispatchQueue.main.async { [weak self] in
                    self?.loadURL("https://demo.nhp")
                }
                return
            }
            
            attempts += 1
            if attempts < maxAttempts {
                DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                    checkAndLoad()
                }
            } else {
                // Timeout - try to load anyway (will show error if NHP required)
                DispatchQueue.main.async { [weak self] in
                    self?.nhpStatusLabel.text = "NHP initialization timeout"
                    self?.loadURL("https://demo.nhp")
                }
            }
        }
        
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
            checkAndLoad()
        }
    }
    
    // MARK: - UI Setup
    private func setupUI() {
        view.backgroundColor = backgroundColor
        
        // URL Bar Container
        let urlBarContainer = UIView()
        urlBarContainer.backgroundColor = backgroundColor
        urlBarContainer.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(urlBarContainer)
        
        // Security Icon
        securityImageView = UIImageView()
        securityImageView.image = UIImage(systemName: "globe")
        securityImageView.tintColor = textSecondary
        securityImageView.contentMode = .scaleAspectFit
        securityImageView.translatesAutoresizingMaskIntoConstraints = false
        urlBarContainer.addSubview(securityImageView)
        
        // URL TextField Container
        let urlFieldContainer = UIView()
        urlFieldContainer.backgroundColor = surfaceColor
        urlFieldContainer.layer.cornerRadius = 20
        urlFieldContainer.layer.borderWidth = 1
        urlFieldContainer.layer.borderColor = nhpBlue.withAlphaComponent(0.3).cgColor
        urlFieldContainer.translatesAutoresizingMaskIntoConstraints = false
        urlBarContainer.addSubview(urlFieldContainer)
        
        // URL TextField
        urlTextField = UITextField()
        urlTextField.placeholder = "Enter URL or search"
        urlTextField.textColor = textPrimary
        urlTextField.font = UIFont.systemFont(ofSize: 14)
        urlTextField.attributedPlaceholder = NSAttributedString(
            string: "Enter URL or search",
            attributes: [.foregroundColor: textSecondary]
        )
        urlTextField.autocapitalizationType = .none
        urlTextField.autocorrectionType = .no
        urlTextField.keyboardType = .URL
        urlTextField.returnKeyType = .go
        urlTextField.delegate = self
        urlTextField.translatesAutoresizingMaskIntoConstraints = false
        urlFieldContainer.addSubview(urlTextField)
        
        // Refresh Button
        refreshButton = createNavButton(systemName: "arrow.clockwise")
        refreshButton.addTarget(self, action: #selector(refreshTapped), for: .touchUpInside)
        urlBarContainer.addSubview(refreshButton)
        
        // QR Scan Button (right of refresh)
        qrScanButton = createNavButton(systemName: "qrcode.viewfinder")
        qrScanButton.addTarget(self, action: #selector(qrScanTapped), for: .touchUpInside)
        urlBarContainer.addSubview(qrScanButton)
        
        // NHP Indicator
        nhpIndicatorView = UIView()
        nhpIndicatorView.backgroundColor = nhpGreen.withAlphaComponent(0.15)
        nhpIndicatorView.isHidden = true
        nhpIndicatorView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(nhpIndicatorView)
        
        let shieldIcon = UIImageView(image: UIImage(systemName: "shield.fill"))
        shieldIcon.tintColor = nhpGreen
        shieldIcon.contentMode = .scaleAspectFit
        shieldIcon.translatesAutoresizingMaskIntoConstraints = false
        nhpIndicatorView.addSubview(shieldIcon)
        
        nhpStatusLabel = UILabel()
        nhpStatusLabel.text = "NHP Protection Enabled"
        nhpStatusLabel.textColor = nhpGreen
        nhpStatusLabel.font = UIFont.monospacedSystemFont(ofSize: 12, weight: .medium)
        nhpStatusLabel.translatesAutoresizingMaskIntoConstraints = false
        nhpIndicatorView.addSubview(nhpStatusLabel)
        
        // Progress View
        progressView = UIProgressView(progressViewStyle: .bar)
        progressView.progressTintColor = nhpBlue
        progressView.trackTintColor = surfaceColor
        progressView.isHidden = true
        progressView.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(progressView)
        
        // WebView Container
        let webViewContainer = UIView()
        webViewContainer.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(webViewContainer)
        
        // Navigation Bar
        navigationBar = UIStackView()
        navigationBar.axis = .horizontal
        navigationBar.distribution = .fillEqually
        navigationBar.alignment = .fill
        navigationBar.backgroundColor = surfaceColor
        navigationBar.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(navigationBar)
        
        backButton = createNavButton(systemName: "chevron.left")
        backButton.addTarget(self, action: #selector(backTapped), for: .touchUpInside)
        backButton.isEnabled = false
        backButton.alpha = 0.3
        
        forwardButton = createNavButton(systemName: "chevron.right")
        forwardButton.addTarget(self, action: #selector(forwardTapped), for: .touchUpInside)
        forwardButton.isEnabled = false
        forwardButton.alpha = 0.3
        
        homeButton = createNavButton(systemName: "house")
        homeButton.addTarget(self, action: #selector(homeTapped), for: .touchUpInside)
        
        tabsButton = createNavButton(systemName: "square.on.square")
        tabsButton.addTarget(self, action: #selector(tabsTapped), for: .touchUpInside)
        
        menuButton = createNavButton(systemName: "ellipsis")
        menuButton.addTarget(self, action: #selector(menuTapped), for: .touchUpInside)
        
        navigationBar.addArrangedSubview(backButton)
        navigationBar.addArrangedSubview(forwardButton)
        navigationBar.addArrangedSubview(homeButton)
        navigationBar.addArrangedSubview(tabsButton)
        navigationBar.addArrangedSubview(menuButton)
        
        // Constraints
        NSLayoutConstraint.activate([
            urlBarContainer.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
            urlBarContainer.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            urlBarContainer.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            urlBarContainer.heightAnchor.constraint(equalToConstant: 56),
            
            securityImageView.leadingAnchor.constraint(equalTo: urlBarContainer.leadingAnchor, constant: 12),
            securityImageView.centerYAnchor.constraint(equalTo: urlBarContainer.centerYAnchor),
            securityImageView.widthAnchor.constraint(equalToConstant: 24),
            securityImageView.heightAnchor.constraint(equalToConstant: 24),
            
            urlFieldContainer.leadingAnchor.constraint(equalTo: securityImageView.trailingAnchor, constant: 8),
            urlFieldContainer.trailingAnchor.constraint(equalTo: refreshButton.leadingAnchor, constant: -8),
            urlFieldContainer.centerYAnchor.constraint(equalTo: urlBarContainer.centerYAnchor),
            urlFieldContainer.heightAnchor.constraint(equalToConstant: 40),
            
            urlTextField.leadingAnchor.constraint(equalTo: urlFieldContainer.leadingAnchor, constant: 16),
            urlTextField.trailingAnchor.constraint(equalTo: urlFieldContainer.trailingAnchor, constant: -16),
            urlTextField.centerYAnchor.constraint(equalTo: urlFieldContainer.centerYAnchor),
            
            refreshButton.trailingAnchor.constraint(equalTo: qrScanButton.leadingAnchor, constant: -4),
            refreshButton.centerYAnchor.constraint(equalTo: urlBarContainer.centerYAnchor),
            refreshButton.widthAnchor.constraint(equalToConstant: 36),
            refreshButton.heightAnchor.constraint(equalToConstant: 36),
            
            qrScanButton.trailingAnchor.constraint(equalTo: urlBarContainer.trailingAnchor, constant: -12),
            qrScanButton.centerYAnchor.constraint(equalTo: urlBarContainer.centerYAnchor),
            qrScanButton.widthAnchor.constraint(equalToConstant: 36),
            qrScanButton.heightAnchor.constraint(equalToConstant: 36),
            
            nhpIndicatorView.topAnchor.constraint(equalTo: urlBarContainer.bottomAnchor),
            nhpIndicatorView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            nhpIndicatorView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            nhpIndicatorView.heightAnchor.constraint(equalToConstant: 32),
            
            shieldIcon.leadingAnchor.constraint(equalTo: nhpIndicatorView.centerXAnchor, constant: -60),
            shieldIcon.centerYAnchor.constraint(equalTo: nhpIndicatorView.centerYAnchor),
            shieldIcon.widthAnchor.constraint(equalToConstant: 16),
            shieldIcon.heightAnchor.constraint(equalToConstant: 16),
            
            nhpStatusLabel.leadingAnchor.constraint(equalTo: shieldIcon.trailingAnchor, constant: 8),
            nhpStatusLabel.centerYAnchor.constraint(equalTo: nhpIndicatorView.centerYAnchor),
            
            progressView.topAnchor.constraint(equalTo: nhpIndicatorView.bottomAnchor),
            progressView.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            progressView.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            progressView.heightAnchor.constraint(equalToConstant: 2),
            
            webViewContainer.topAnchor.constraint(equalTo: progressView.bottomAnchor),
            webViewContainer.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            webViewContainer.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            webViewContainer.bottomAnchor.constraint(equalTo: navigationBar.topAnchor),
            
            navigationBar.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            navigationBar.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            navigationBar.bottomAnchor.constraint(equalTo: view.safeAreaLayoutGuide.bottomAnchor),
            navigationBar.heightAnchor.constraint(equalToConstant: 52),
        ])
        
        self.webViewContainer = webViewContainer
    }
    
    private var webViewContainer: UIView!
    
    private func createNavButton(systemName: String) -> UIButton {
        let button = UIButton(type: .system)
        button.setImage(UIImage(systemName: systemName), for: .normal)
        button.tintColor = textSecondary
        button.translatesAutoresizingMaskIntoConstraints = false
        return button
    }
    
    private func setupWebView() {
        let config = WKWebViewConfiguration()
        config.allowsInlineMediaPlayback = true
        config.mediaTypesRequiringUserActionForPlayback = []
        config.applicationNameForUserAgent = "StealthDNS/1.0"
        
        webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.allowsBackForwardNavigationGestures = true
        webView.backgroundColor = backgroundColor
        webView.scrollView.backgroundColor = backgroundColor
        webView.translatesAutoresizingMaskIntoConstraints = false
        
        webViewContainer.addSubview(webView)
        
        NSLayoutConstraint.activate([
            webView.topAnchor.constraint(equalTo: webViewContainer.topAnchor),
            webView.leadingAnchor.constraint(equalTo: webViewContainer.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: webViewContainer.trailingAnchor),
            webView.bottomAnchor.constraint(equalTo: webViewContainer.bottomAnchor),
        ])
    }
    
    private func setupObservers() {
        progressObservation = webView.observe(\.estimatedProgress, options: [.new]) { [weak self] webView, _ in
            DispatchQueue.main.async {
                let progress = Float(webView.estimatedProgress)
                self?.progressView.progress = progress
                self?.progressView.isHidden = progress >= 1.0
            }
        }
    }
    
    // MARK: - Tab Management
    
    // Create new tab in background without switching
    private func createNewTabInBackground() {
        saveCurrentTabState()
        
        let newTab = BrowserTab(
            id: tabIdCounter,
            title: "New Tab",
            url: "https://demo.nhp",
            isNhp: false,
            webViewData: nil
        )
        tabIdCounter += 1
        tabs.append(newTab)
        // Don't change currentTabIndex - stay on current tab
    }
    
    // Create new tab and switch to it
    private func createNewTabAndSwitch() {
        saveCurrentTabState()
        
        let newTab = BrowserTab(
            id: tabIdCounter,
            title: "New Tab",
            url: "https://demo.nhp",
            isNhp: false,
            webViewData: nil
        )
        tabIdCounter += 1
        tabs.append(newTab)
        currentTabIndex = tabs.count - 1
        
        // Load home page for new tab
        currentPageIsNhp = false
        nhpIndicatorView.isHidden = true
        urlTextField.text = ""
        
        webView.stopLoading()
        if let url = URL(string: "https://demo.nhp") {
            webView.load(URLRequest(url: url))
        }
    }
    
    // Used only for initial tab creation
    @discardableResult
    private func createNewTab() -> BrowserTab {
        let newTab = BrowserTab(
            id: tabIdCounter,
            title: "New Tab",
            url: "https://demo.nhp",
            isNhp: false,
            webViewData: nil
        )
        tabIdCounter += 1
        tabs.append(newTab)
        currentTabIndex = tabs.count - 1
        return newTab
    }
    
    private func saveCurrentTabState() {
        guard !tabs.isEmpty, currentTabIndex < tabs.count else { return }
        
        tabs[currentTabIndex].title = webView.title ?? "New Tab"
        tabs[currentTabIndex].url = webView.url?.absoluteString ?? "about:blank"
        tabs[currentTabIndex].isNhp = currentPageIsNhp
    }
    
    private func switchToTab(_ index: Int) {
        guard index != currentTabIndex, index < tabs.count else { return }
        
        // Save current tab state first
        saveCurrentTabState()
        
        // Switch to new tab
        currentTabIndex = index
        let tab = tabs[index]
        
        // Stop current loading and load the tab's URL
        webView.stopLoading()
        
        // Check if URL was loaded via NHP (tab.url is display URL, might be NHP URL)
        if tab.isNhp || nhpUrlMapping.values.contains(tab.url) {
            currentPageIsNhp = true
            currentNhpDisplayUrl = tab.url
        } else {
            currentPageIsNhp = false
            currentNhpDisplayUrl = nil
        }
        urlTextField.text = tab.url
        
        // Load the URL - use loadURL to handle NHP domains properly
        let urlString = tab.url
        if !urlString.isEmpty && urlString != "about:blank" {
            loadURL(urlString)
        } else {
            loadURL("https://demo.nhp")
        }
        
        showToast("Switched to: \(tab.title)")
    }
    
    private func closeCurrentTab() {
        guard tabs.count > 1 else {
            showToast("At least one tab required")
            return
        }
        
        let alert = UIAlertController(title: "Close Tab", message: "Close this tab?\n\n\(tabs[currentTabIndex].title)", preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        alert.addAction(UIAlertAction(title: "Close", style: .destructive) { [weak self] _ in
            guard let self = self else { return }
            
            self.tabs.remove(at: self.currentTabIndex)
            if self.currentTabIndex >= self.tabs.count {
                self.currentTabIndex = self.tabs.count - 1
            }
            
            // Load the new current tab
            let tab = self.tabs[self.currentTabIndex]
            self.webView.stopLoading()
            
            // Check if URL was loaded via NHP
            if tab.isNhp || self.nhpUrlMapping.values.contains(tab.url) {
                self.currentPageIsNhp = true
                self.currentNhpDisplayUrl = tab.url
            } else {
                self.currentPageIsNhp = false
                self.currentNhpDisplayUrl = nil
            }
            self.urlTextField.text = tab.url
            
            // Load the URL - use loadURL to handle NHP domains properly
            let urlString = tab.url
            if !urlString.isEmpty && urlString != "about:blank" {
                self.loadURL(urlString)
            } else {
                self.loadURL("https://demo.nhp")
            }
            
            self.showToast("Tab closed, \(self.tabs.count) remaining")
        })
        present(alert, animated: true)
    }
    
    // MARK: - Actions
    @objc private func backTapped() {
        if webView.canGoBack {
            webView.goBack()
        }
    }
    
    @objc private func forwardTapped() {
        if webView.canGoForward {
            webView.goForward()
        }
    }
    
    @objc private func refreshTapped() {
        webView.reload()
    }
    
    @objc private func qrScanTapped() {
        openQRScanner()
    }
    
    @objc private func homeTapped() {
        currentPageIsNhp = false
        nhpIndicatorView.isHidden = true
        loadURL("https://demo.nhp")
    }
    
    @objc private func tabsTapped() {
        saveCurrentTabState()
        showTabsMenu()
    }
    
    @objc private func menuTapped() {
        showMainMenu()
    }
    
    // MARK: - Tabs Menu
    private func showTabsMenu() {
        let alert = UIAlertController(title: "Tabs (\(tabs.count))", message: nil, preferredStyle: .actionSheet)
        
        // Add tab items
        for (index, tab) in tabs.enumerated() {
            let prefix = index == currentTabIndex ? "▶ " : "   "
            let nhpBadge = tab.isNhp ? " 🛡️" : ""
            let title = String(tab.title.prefix(25)) + (tab.title.count > 25 ? "..." : "")
            
            alert.addAction(UIAlertAction(title: "\(prefix)\(title)\(nhpBadge)", style: .default) { [weak self] _ in
                self?.switchToTab(index)
            })
        }
        
        alert.addAction(UIAlertAction(title: "➕ New Tab", style: .default) { [weak self] _ in
            self?.createNewTabInBackground()
            self?.showToast("New tab created (\(self?.tabs.count ?? 0))")
        })
        
        alert.addAction(UIAlertAction(title: "🗑️ Close Current Tab", style: .destructive) { [weak self] _ in
            self?.closeCurrentTab()
        })
        
        alert.addAction(UIAlertAction(title: "🧹 Clear Browsing Data", style: .destructive) { [weak self] _ in
            self?.clearBrowsingData()
        })
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        
        if let popover = alert.popoverPresentationController {
            popover.sourceView = tabsButton
            popover.sourceRect = tabsButton.bounds
        }
        
        present(alert, animated: true)
    }
    
    // MARK: - Main Menu
    private func showMainMenu() {
        let alert = UIAlertController(title: "Menu", message: nil, preferredStyle: .actionSheet)
        
        alert.addAction(UIAlertAction(title: "📷 Scan QR Code", style: .default) { [weak self] _ in
            self?.openQRScanner()
        })
        
        alert.addAction(UIAlertAction(title: "🔗 Share Page", style: .default) { [weak self] _ in
            self?.shareCurrentPage()
        })
        
        alert.addAction(UIAlertAction(title: "⭐ Add Bookmark", style: .default) { [weak self] _ in
            self?.addBookmark()
        })
        
        alert.addAction(UIAlertAction(title: "📚 View Bookmarks", style: .default) { [weak self] _ in
            self?.showBookmarks()
        })
        
        alert.addAction(UIAlertAction(title: "📋 Copy Link", style: .default) { [weak self] _ in
            self?.copyCurrentURL()
        })
        
        alert.addAction(UIAlertAction(title: "🔍 Find in Page", style: .default) { [weak self] _ in
            self?.showFindInPage()
        })
        
        alert.addAction(UIAlertAction(title: "🖥️ Desktop Site", style: .default) { [weak self] _ in
            self?.toggleDesktopMode()
        })
        
        alert.addAction(UIAlertAction(title: "ℹ️ About StealthDNS", style: .default) { [weak self] _ in
            self?.showAbout()
        })
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        
        if let popover = alert.popoverPresentationController {
            popover.sourceView = menuButton
            popover.sourceRect = menuButton.bounds
        }
        
        present(alert, animated: true)
    }
    
    // MARK: - Menu Actions
    private func shareCurrentPage() {
        guard let url = webView.url else { return }
        let title = webView.title ?? "StealthDNS"
        
        let activityVC = UIActivityViewController(activityItems: [title, url], applicationActivities: nil)
        if let popover = activityVC.popoverPresentationController {
            popover.sourceView = menuButton
            popover.sourceRect = menuButton.bounds
        }
        present(activityVC, animated: true)
    }
    
    private func addBookmark() {
        guard let url = webView.url?.absoluteString else { return }
        let title = webView.title ?? "Untitled"
        
        var bookmarks = UserDefaults.standard.stringArray(forKey: "bookmarks") ?? []
        
        // Check if already bookmarked
        if bookmarks.contains(where: { $0.contains("|\(url)") }) {
            showToast("Page already bookmarked")
            return
        }
        
        bookmarks.append("\(title)|\(url)")
        UserDefaults.standard.set(bookmarks, forKey: "bookmarks")
        
        showToast("Bookmark added: \(title)")
    }
    
    private func showBookmarks() {
        let bookmarks = UserDefaults.standard.stringArray(forKey: "bookmarks") ?? []
        
        if bookmarks.isEmpty {
            showToast("No bookmarks")
            return
        }
        
        let alert = UIAlertController(title: "Bookmarks (\(bookmarks.count))", message: nil, preferredStyle: .actionSheet)
        
        for bookmark in bookmarks {
            let parts = bookmark.split(separator: "|", maxSplits: 1).map(String.init)
            let title = parts.first ?? "Untitled"
            let url = parts.count > 1 ? parts[1] : ""
            
            let displayTitle = String(title.prefix(35)) + (title.count > 35 ? "..." : "")
            
            alert.addAction(UIAlertAction(title: displayTitle, style: .default) { [weak self] _ in
                self?.loadURL(url)
            })
        }
        
        alert.addAction(UIAlertAction(title: "🗑️ Manage Bookmarks", style: .destructive) { [weak self] _ in
            self?.showBookmarkManager(bookmarks)
        })
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        
        if let popover = alert.popoverPresentationController {
            popover.sourceView = menuButton
            popover.sourceRect = menuButton.bounds
        }
        
        present(alert, animated: true)
    }
    
    private func showBookmarkManager(_ bookmarks: [String]) {
        let alert = UIAlertController(title: "Delete Bookmarks", message: nil, preferredStyle: .actionSheet)
        
        for (index, bookmark) in bookmarks.enumerated() {
            let parts = bookmark.split(separator: "|", maxSplits: 1).map(String.init)
            let title = parts.first ?? "Untitled"
            let displayTitle = "🗑️ " + String(title.prefix(30)) + (title.count > 30 ? "..." : "")
            
            alert.addAction(UIAlertAction(title: displayTitle, style: .destructive) { [weak self] _ in
                var current = UserDefaults.standard.stringArray(forKey: "bookmarks") ?? []
                current.remove(at: index)
                UserDefaults.standard.set(current, forKey: "bookmarks")
                self?.showToast("Bookmark deleted")
            })
        }
        
        alert.addAction(UIAlertAction(title: "Clear All Bookmarks", style: .destructive) { [weak self] _ in
            UserDefaults.standard.removeObject(forKey: "bookmarks")
            self?.showToast("All bookmarks cleared")
        })
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        
        if let popover = alert.popoverPresentationController {
            popover.sourceView = menuButton
            popover.sourceRect = menuButton.bounds
        }
        
        present(alert, animated: true)
    }
    
    private func copyCurrentURL() {
        guard let url = webView.url?.absoluteString else { return }
        UIPasteboard.general.string = url
        showToast("Link copied")
    }
    
    private func showFindInPage() {
        let alert = UIAlertController(title: "Find in Page", message: nil, preferredStyle: .alert)
        alert.addTextField { textField in
            textField.placeholder = "Enter search text"
        }
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        alert.addAction(UIAlertAction(title: "Find", style: .default) { [weak self] _ in
            guard let query = alert.textFields?.first?.text, !query.isEmpty else { return }
            let js = "window.find('\(query)', false, false, true)"
            self?.webView.evaluateJavaScript(js, completionHandler: nil)
        })
        
        present(alert, animated: true)
    }
    
    private var isDesktopMode = false
    
    private func toggleDesktopMode() {
        isDesktopMode.toggle()
        
        if isDesktopMode {
            webView.customUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
        } else {
            webView.customUserAgent = nil
        }
        
        webView.reload()
        showToast(isDesktopMode ? "Desktop mode enabled" : "Mobile mode enabled")
    }
    
    private func openQRScanner() {
        let qrScanVC = QRScanViewController()
        qrScanVC.modalPresentationStyle = .fullScreen
        present(qrScanVC, animated: true)
    }

    private func showAbout() {
        let nhpStatus = NhpcoreIsInitialized() ? "Initialized ✓" : "Not initialized"
        let message = """
        StealthDNS Browser
        Version: 1.0.0
        
        A secure browser based on NHP (Network Hiding Protocol)
        
        NHP Status: \(nhpStatus)
        
        © 2024 OpenNHP
        """
        
        let alert = UIAlertController(title: "About", message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "OK", style: .default))
        present(alert, animated: true)
    }
    
    private func clearBrowsingData() {
        let alert = UIAlertController(title: "Clear Browsing Data", message: "Clear all browsing data?\n\nThis will clear:\n• Browsing history\n• Cache\n• Cookies\n• All tabs", preferredStyle: .alert)
        
        alert.addAction(UIAlertAction(title: "Cancel", style: .cancel))
        alert.addAction(UIAlertAction(title: "Clear", style: .destructive) { [weak self] _ in
            // Clear WebView data
            let dataStore = WKWebsiteDataStore.default()
            let dataTypes = WKWebsiteDataStore.allWebsiteDataTypes()
            dataStore.removeData(ofTypes: dataTypes, modifiedSince: Date.distantPast) { }
            
            // Reset state
            self?.nhpUrlMapping.removeAll()
            self?.currentPageIsNhp = false
            self?.currentNhpDisplayUrl = nil
            self?.nhpIndicatorView.isHidden = true
            
            // Reset tabs
            self?.tabs.removeAll()
            self?.tabIdCounter = 0
            self?.createNewTab()
            self?.loadURL("https://demo.nhp")
            
            self?.showToast("Browsing data cleared")
        })
        
        present(alert, animated: true)
    }
    
    // MARK: - URL Loading
    func loadURL(_ input: String) {
        var urlString = input.trimmingCharacters(in: .whitespacesAndNewlines)
        
        guard !urlString.isEmpty else { return }
        
        // Reset NHP status
        currentPageIsNhp = false
        currentNhpDisplayUrl = nil
        nhpIndicatorView.isHidden = true
        
        // Check if it's a search query or URL
        if !urlString.contains(".") || urlString.contains(" ") {
            urlString = "https://demo.nhp/s?wd=" + (urlString.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? "")
        } else if !urlString.hasPrefix("http://") && !urlString.hasPrefix("https://") {
            urlString = "https://" + urlString
        }
        
        guard let url = URL(string: urlString) else { return }
        let host = url.host ?? ""
        
        // Check if it's an NHP domain
        if NhpcoreIsNHPDomain(host) {
            processNHPURL(urlString, host: host)
        } else {
            webView.load(URLRequest(url: url))
        }
    }
    
    private func processNHPURL(_ originalURL: String, host: String) {
        progressView.isHidden = false
        progressView.progress = 0.1
        nhpIndicatorView.isHidden = false
        nhpStatusLabel.text = "Performing NHP knock..."
        
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            let resourceId = NhpcoreExtractResourceID(host)
            let resultJSON = NhpcoreGetKnockResultJSON(resourceId)
            
            DispatchQueue.main.async {
                self?.handleKnockResult(resultJSON, originalURL: originalURL, host: host)
            }
        }
    }
    
    private func handleKnockResult(_ resultJSON: String, originalURL: String, host: String) {
        guard let data = resultJSON.data(using: .utf8),
              let result = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            showError("Cannot parse NHP response")
            nhpIndicatorView.isHidden = true
            progressView.isHidden = true
            return
        }
        
        // Parse ServerKnockAckMsg format
        let errCode = result["errCode"] as? String ?? ""
        let errMsg = result["errMsg"] as? String ?? ""
        let resHost = result["resHost"] as? [String: String]
        
        // Success if errCode is "0" or empty and resHost has values
        let success = (errCode == "0" || errCode.isEmpty) && resHost != nil && !(resHost?.isEmpty ?? true)
        
        // Get first resource host from map
        let resolvedHost = resHost?.values.first ?? ""
        
        guard success, !resolvedHost.isEmpty else {
            let errorMessage = errMsg.isEmpty ? "Knock failed (error: \(errCode))" : errMsg
            showError("NHP knock failed: \(errorMessage)")
            nhpIndicatorView.isHidden = true
            progressView.isHidden = true
            currentPageIsNhp = false
            return
        }
        
        // Replace host in URL and load
        let processedURL = originalURL.replacingOccurrences(of: host, with: resolvedHost)
        currentPageIsNhp = true
        
        // Save mapping: real URL -> original NHP URL (for back/forward navigation)
        nhpUrlMapping[processedURL] = originalURL
        
        // Save the original NHP URL to display in address bar
        currentNhpDisplayUrl = originalURL
        
        nhpStatusLabel.text = "NHP Protection Enabled"
        
        // Update security icon
        securityImageView.image = UIImage(systemName: "shield.fill")
        securityImageView.tintColor = nhpGreen
        
        // Update address bar to show original NHP URL
        urlTextField.text = originalURL
        
        if let url = URL(string: processedURL) {
            webView.load(URLRequest(url: url))
        }
    }
    
    private func showError(_ message: String) {
        let alert = UIAlertController(title: "Error", message: message, preferredStyle: .alert)
        alert.addAction(UIAlertAction(title: "OK", style: .default))
        present(alert, animated: true)
    }
    
    private func showToast(_ message: String) {
        let toast = UILabel()
        toast.text = message
        toast.textColor = .white
        toast.backgroundColor = UIColor.black.withAlphaComponent(0.7)
        toast.textAlignment = .center
        toast.font = UIFont.systemFont(ofSize: 14)
        toast.layer.cornerRadius = 8
        toast.clipsToBounds = true
        toast.translatesAutoresizingMaskIntoConstraints = false
        
        view.addSubview(toast)
        
        NSLayoutConstraint.activate([
            toast.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            toast.bottomAnchor.constraint(equalTo: navigationBar.topAnchor, constant: -20),
            toast.widthAnchor.constraint(lessThanOrEqualTo: view.widthAnchor, constant: -40),
            toast.heightAnchor.constraint(equalToConstant: 36)
        ])
        
        toast.layoutIfNeeded()
        toast.frame.size.width += 32
        
        UIView.animate(withDuration: 0.3, delay: 1.5, options: [], animations: {
            toast.alpha = 0
        }) { _ in
            toast.removeFromSuperview()
        }
    }
    
    private func updateNavigationButtons() {
        backButton.isEnabled = webView.canGoBack
        backButton.alpha = webView.canGoBack ? 1.0 : 0.3
        forwardButton.isEnabled = webView.canGoForward
        forwardButton.alpha = webView.canGoForward ? 1.0 : 0.3
    }
    
    private func updateSecurityIndicator(for url: URL?) {
        guard let url = url else { return }
        
        // Check if this URL was loaded via NHP knock (for back/forward navigation)
        if let nhpUrl = nhpUrlMapping[url.absoluteString] {
            currentPageIsNhp = true
            currentNhpDisplayUrl = nhpUrl
        }
        
        if currentPageIsNhp {
            securityImageView.image = UIImage(systemName: "shield.fill")
            securityImageView.tintColor = nhpGreen
            nhpIndicatorView.isHidden = false
            nhpStatusLabel.text = "NHP Protection Enabled"
        } else {
            nhpIndicatorView.isHidden = true
            
            if url.scheme == "https" {
                securityImageView.image = UIImage(systemName: "lock.fill")
                securityImageView.tintColor = nhpGreen
            } else {
                securityImageView.image = UIImage(systemName: "lock.open.fill")
                securityImageView.tintColor = nhpOrange
            }
        }
    }
}

// MARK: - UITextFieldDelegate
extension BrowserViewController: UITextFieldDelegate {
    func textFieldShouldReturn(_ textField: UITextField) -> Bool {
        textField.resignFirstResponder()
        if let text = textField.text {
            loadURL(text)
        }
        return true
    }
}

// MARK: - WKNavigationDelegate
extension BrowserViewController: WKNavigationDelegate {
    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
        progressView.isHidden = false
        progressView.progress = 0.1
        
        if let url = webView.url {
            let urlString = url.absoluteString
            
            // Check if this URL was loaded via NHP knock (for back/forward navigation)
            if let nhpUrl = nhpUrlMapping[urlString] {
                // Restore NHP state when navigating back to NHP page
                currentPageIsNhp = true
                currentNhpDisplayUrl = nhpUrl
                urlTextField.text = nhpUrl
            } else if currentPageIsNhp, let nhpUrl = currentNhpDisplayUrl {
                // Still in NHP mode (e.g., redirect within NHP page)
                urlTextField.text = nhpUrl
            } else {
                urlTextField.text = urlString
            }
            
            updateSecurityIndicator(for: url)
        }
    }
    
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        progressView.isHidden = true
        updateNavigationButtons()
        
        if let url = webView.url {
            let urlString = url.absoluteString
            let displayUrl: String
            
            // Check if this URL was loaded via NHP knock (for back/forward navigation)
            if let nhpUrl = nhpUrlMapping[urlString] {
                // Restore NHP state when navigating back to NHP page
                currentPageIsNhp = true
                currentNhpDisplayUrl = nhpUrl
                displayUrl = nhpUrl
            } else if currentPageIsNhp, let nhpUrl = currentNhpDisplayUrl {
                // Still in NHP mode (e.g., redirect within NHP page)
                // Also save this redirected URL to mapping for back/forward navigation
                nhpUrlMapping[urlString] = nhpUrl
                print("Added redirect URL to mapping: \(urlString) -> \(nhpUrl)")
                displayUrl = nhpUrl
            } else {
                // Normal non-NHP page
                currentPageIsNhp = false
                currentNhpDisplayUrl = nil
                displayUrl = urlString
            }
            
            urlTextField.text = displayUrl
            
            // Update current tab info
            if !tabs.isEmpty, currentTabIndex < tabs.count {
                tabs[currentTabIndex].title = webView.title ?? "New Tab"
                tabs[currentTabIndex].url = displayUrl  // Store the display URL (NHP URL)
                tabs[currentTabIndex].isNhp = currentPageIsNhp
            }
        }
    }
    
    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        progressView.isHidden = true
        currentPageIsNhp = false
        nhpIndicatorView.isHidden = true
        
        let nsError = error as NSError
        // Ignore cancelled errors (user navigated away)
        if nsError.code == NSURLErrorCancelled { return }
        
        showErrorPage(error: error.localizedDescription, url: webView.url?.absoluteString ?? "")
    }
    
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        progressView.isHidden = true
        currentPageIsNhp = false
        nhpIndicatorView.isHidden = true
        
        let nsError = error as NSError
        
        // Ignore certain errors that are not real page load failures
        switch nsError.code {
        case NSURLErrorCancelled:
            // User navigated away or cancelled
            return
        case NSURLErrorUnsupportedURL:
            // Unsupported URL scheme (like intent://)
            return
        case 102:
            // Frame load interrupted (normal for redirects)
            return
        case 204:
            // Plug-in handled load
            return
        default:
            break
        }
        
        showErrorPage(error: error.localizedDescription, url: webView.url?.absoluteString ?? "")
    }
    
    private func showErrorPage(error: String, url: String) {
        let errorHtml = """
        <html>
        <head>
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <style>
                body { 
                    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
                    background: #0b1020; 
                    color: #fff; 
                    display: flex; 
                    flex-direction: column;
                    align-items: center; 
                    justify-content: center; 
                    height: 100vh; 
                    margin: 0;
                    padding: 20px;
                    box-sizing: border-box;
                    text-align: center;
                }
                .icon { font-size: 64px; margin-bottom: 20px; }
                h1 { font-size: 20px; margin: 0 0 10px 0; color: #ff9800; }
                p { font-size: 14px; color: #9e9e9e; margin: 5px 0; word-break: break-all; }
                .url { font-size: 12px; color: #666; margin-top: 15px; max-width: 100%; overflow: hidden; text-overflow: ellipsis; }
                button { 
                    margin-top: 20px; 
                    padding: 12px 24px; 
                    background: #29b6f6; 
                    border: none; 
                    border-radius: 8px; 
                    color: white; 
                    font-size: 16px;
                    cursor: pointer;
                }
            </style>
        </head>
        <body>
            <div class="icon">🌐</div>
            <h1>Page Load Failed</h1>
            <p>Error: \(error)</p>
            <p class="url">\(url)</p>
            <button onclick="window.location.href='https://demo.nhp'">Go Back</button>
        </body>
        </html>
        """
        webView.loadHTMLString(errorHtml, baseURL: nil)
    }
    
    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction, decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard let url = navigationAction.request.url else {
            decisionHandler(.allow)
            return
        }
        
        let scheme = url.scheme?.lowercased() ?? ""
        let host = url.host ?? ""
        
        // Allow all standard web protocols to be handled by WKWebView
        if scheme == "http" || scheme == "https" {
            // Check if it's an NHP domain
            if !host.isEmpty && NhpcoreIsNHPDomain(host) {
                decisionHandler(.cancel)
                processNHPURL(url.absoluteString, host: host)
                return
            }
            
            // For normal HTTP/HTTPS URLs, let WKWebView handle them
            if navigationAction.navigationType == .linkActivated {
                currentPageIsNhp = false
                currentNhpDisplayUrl = nil
            }
            decisionHandler(.allow)
            return
        }
        
        // Ignore special schemes (javascript, about, data, blob)
        if scheme == "javascript" || scheme == "about" || scheme == "data" || scheme == "blob" {
            decisionHandler(.allow)
            return
        }
        
        // Handle external app URLs (tel, mailto, sms, etc.)
        if scheme == "tel" || scheme == "mailto" || scheme == "sms" || scheme == "facetime" {
            if UIApplication.shared.canOpenURL(url) {
                UIApplication.shared.open(url, options: [:], completionHandler: nil)
            }
            decisionHandler(.cancel)
            return
        }
        
        // Handle custom app schemes (baiduboxapp, weixin, alipay, etc.)
        // Try to open the app if installed, otherwise silently ignore
        if !scheme.isEmpty && scheme != "file" {
            print("Custom scheme detected: \(scheme)")
            if UIApplication.shared.canOpenURL(url) {
                UIApplication.shared.open(url, options: [:], completionHandler: nil)
            } else {
                // App not installed, silently ignore
                print("No app installed for scheme: \(scheme)")
            }
            // Always cancel to prevent WKWebView from showing error
            decisionHandler(.cancel)
            return
        }
        
        // For any other cases, let WKWebView handle
        decisionHandler(.allow)
    }
}

// MARK: - WKUIDelegate
extension BrowserViewController: WKUIDelegate {
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration, for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if navigationAction.targetFrame == nil {
            webView.load(navigationAction.request)
        }
        return nil
    }
}
