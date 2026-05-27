# CC-Box 图标候选

这些是原创 SVG 候选稿，供选择方向用；未替换现有 exe 图标或托盘图标。

每套包含：

- `app.svg`：适合作为应用 / exe 主图标。
- `tray.svg`：适合作为右下角托盘基础图标，细节更少，便于缩小到 16×16 或 24×24。

候选方向：

1. `01-vault-sync`：工具箱 + 加密锁 + 同步箭头，强调“安全备份和同步”。
2. `02-bridge-box`：双端连接 + 双向同步，强调“多设备桥接”。
3. `03-crystal-cube`：晶体盒子 + 锁，强调“加密存储和版本保险箱”。
4. `04-minimal-orbit`：极简盒子 + 环形同步，强调“小而清晰、跨平台统一”。

可以先打开 `preview.svg` 看整体对比。选好方向后，再把对应方案细化成正式尺寸：

- Windows `.ico`
- Wails `build/appicon.png`
- 托盘 `icon.ico` / `icon_synced.ico` / `icon_pending.ico` / `icon_syncing.ico` / `icon_conflict.ico`
