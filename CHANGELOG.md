# Changelog

## 1.0.0 (2026-05-18)


### Features

* add admin privilege hint next to start button with i18n support ([e0f8def](https://github.com/OpenNHP/stealth-dns/commit/e0f8def1ac9adc30cb3cab9b32b7d74f8b01f257))
* add DNS configuration setting and restoration under DHCP mode on Windows OS ([de452fa](https://github.com/OpenNHP/stealth-dns/commit/de452fa7315d20ada4cfe8008eaa7dfd33db4121))
* add Open https://demo.nhp button to status panel ([4dbb82e](https://github.com/OpenNHP/stealth-dns/commit/4dbb82e34385ad4ea579105e17ceaf4e7ab8fab4))
* add root certificate trust and certificate issuance functions ([1246576](https://github.com/OpenNHP/stealth-dns/commit/1246576273eedfcf38af1869fbd316e090733582))
* add StealthDNS browser based on OpenNHP.Enable secure browsing on Android/iOS ([5295e39](https://github.com/OpenNHP/stealth-dns/commit/5295e39fb189e08fa13abe6417708b60a1b422b0))
* add StealthDNS client DNS proxy service based on OpenNHP ([9c0dae2](https://github.com/OpenNHP/stealth-dns/commit/9c0dae20185c4016e6c5d85ceb98a989e4adef46))
* add StealthDNS client DNS proxy service based on OpenNHP ([ef27f28](https://github.com/OpenNHP/stealth-dns/commit/ef27f28e6b0e7fd477ba5283d00117af82ead212))
* Add StealthDNS UI feature ([d16a02a](https://github.com/OpenNHP/stealth-dns/commit/d16a02a4b3dbe131ee0c5101840c3ae51dbe6183))
* Add StealthDNS UI feature ([a0b6ef0](https://github.com/OpenNHP/stealth-dns/commit/a0b6ef00b9cb32a34a7e26068391507d4a327be7))
* add warning when StealthDNS service is run without admin/root privileges ([0d52c00](https://github.com/OpenNHP/stealth-dns/commit/0d52c0019f5a4ee17dc71b48818000b1f5819a1b))
* integrate opennhp as git submodule for auto SDK build ([d7d3671](https://github.com/OpenNHP/stealth-dns/commit/d7d36716e6ee1352736a014e58f87776f447c932))
* show DHCP indicator for DNS addresses on Windows ([cf14334](https://github.com/OpenNHP/stealth-dns/commit/cf143349e2f15344f7835098f226f502fbd43c38))


### Bug Fixes

* /simplify round — bump sibling Go modules + desktop docs ([e7d6cfd](https://github.com/OpenNHP/stealth-dns/commit/e7d6cfd16540b61f84ac8b60de8586bbc8a0178f))
* add BuildTime to version ldflags for Windows build ([bfb26c8](https://github.com/OpenNHP/stealth-dns/commit/bfb26c8a01757b4700540ee5c8c96248b9e83194))
* Added handling for custom app schemes to prevent inaccessible blank links when clicked; fixed APP icon display. ([090970a](https://github.com/OpenNHP/stealth-dns/commit/090970a35328a16cda7d3fe4bae9804184ef130e))
* avoid admin privilege for stopping service when signal file method succeeds ([7a9361f](https://github.com/OpenNHP/stealth-dns/commit/7a9361f35656ac717a117216f11c6589abea0864))
* bump Go to 1.26.2 for stdlib CVE fixes ([7388b32](https://github.com/OpenNHP/stealth-dns/commit/7388b32c8023a58839c4ddd80c91ac3a606d8653))
* bump Go to 1.26.2 to pick up stdlib CVEs ([#1144](https://github.com/OpenNHP/stealth-dns/issues/1144) part 1/N) ([f2d030e](https://github.com/OpenNHP/stealth-dns/commit/f2d030ecf054b554a5ceff4a3deb8ab50581684c))
* create cert directory before copying rootCA.pem in build.bat ([b9ba9a7](https://github.com/OpenNHP/stealth-dns/commit/b9ba9a7fd81ab45d335bb74ed8554c3735d655d4))
* detect DHCP without requiring interfaceName ([17f24f6](https://github.com/OpenNHP/stealth-dns/commit/17f24f6b947829372d604e2f963b43478bfb2700))
* DNS proxy configuration failure on Chinese Win11 ([735898c](https://github.com/OpenNHP/stealth-dns/commit/735898c107bd0753fb9b86cfbbd07e52891bc758))
* Fix wails command path and directory creation in build process ([d4587a6](https://github.com/OpenNHP/stealth-dns/commit/d4587a6cf7ce9d51185fd4f78277dce288228292))
* Ignore submodule changes and improve restore logic ([de75b7d](https://github.com/OpenNHP/stealth-dns/commit/de75b7d93790d6f7bb2f268216a66a3034dbaba6))
* improve stop service to avoid unnecessary admin privilege request ([6168b21](https://github.com/OpenNHP/stealth-dns/commit/6168b215e12a179ac32df5c2231655152eae8d9a))
* **landing:** maintain demo.nhp after knock and set as default ([3cd1066](https://github.com/OpenNHP/stealth-dns/commit/3cd1066b587028a24ab5aa7836d8f5b81bf53fed))
* **landing:** maintain demo.nhp after knock and set as default ([acadb13](https://github.com/OpenNHP/stealth-dns/commit/acadb13e4a44139dc4e756388dc09fb76d2d0274))
* **landing:** maintain demo.nhp after knock and set as default app homepage ([ac6b5c1](https://github.com/OpenNHP/stealth-dns/commit/ac6b5c1e3de4d7dbb260faa767e627b4e67df427))
* prevent build from modifying tracked files ([cb384c7](https://github.com/OpenNHP/stealth-dns/commit/cb384c786b0e3d6e13a6d8898838d0958f7da34a))
* Prevent build process from modifying tracked files ([52c4bc6](https://github.com/OpenNHP/stealth-dns/commit/52c4bc6dfb7d37795fb8efd4d5b8b1c978a55792))
* sync version ldflags with Makefile for Windows build ([c63bbc4](https://github.com/OpenNHP/stealth-dns/commit/c63bbc4a37d780777263111f23f227a3768cf524))
