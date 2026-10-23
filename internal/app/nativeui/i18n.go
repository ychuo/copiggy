package nativeui

import (
	"fmt"
)

var supportedLanguages = []struct{ Code, Name string }{
	{"en", "English"}, {"zh-Hans", "简体中文"}, {"zh-Hant", "繁體中文"}, {"ja", "日本語"},
}

// Catalog keys and the first column are concise English product copy.
// The remaining columns follow supportedLanguages; native control IDs stay separate.
var translations = map[string][4]string{
	"Email":                  {"Email", "邮箱", "電子郵件", "メール"},
	"Email address":          {"Email address", "邮箱地址", "電子郵件地址", "メールアドレス"},
	"Phone":                  {"Phone", "电话", "電話", "電話番号"},
	"Coordinates":            {"Coordinates", "经纬度", "經緯度", "座標"},
	"Date":                   {"Date", "日期", "日期", "日付"},
	"Timestamp":              {"Timestamp", "时间戳", "時間戳", "タイムスタンプ"},
	"Color":                  {"Color", "颜色", "顏色", "色"},
	"File path":              {"File path", "文件路径", "檔案路徑", "ファイルパス"},
	"Verification code":      {"Verification code", "验证码", "驗證碼", "認証コード"},
	"Token":                  {"Token", "令牌", "權杖", "トークン"},
	"Private key":            {"Private key", "私钥", "私鑰", "秘密鍵"},
	"JSON":                   {"JSON", "JSON", "JSON", "JSON"},
	"Plain text":             {"Plain text", "纯文本", "純文字", "プレーンテキスト"},
	"Image":                  {"Image", "图片", "圖片", "画像"},
	"International":          {"International", "国际号码", "國際號碼", "国際番号"},
	"%d digits":              {"%d digits", "%d 位", "%d 碼", "%d 桁"},
	"%d properties":          {"%d properties", "%d 个属性", "%d 個屬性", "%d 項目"},
	"Hidden content":         {"Hidden content", "内容已隐藏", "內容已隱藏", "非表示の内容"},
	"Mail":                   {"Mail", "邮件", "郵件", "メール"},
	"Call":                   {"Call", "拨号", "撥號", "発信"},
	"Maps":                   {"Maps", "地图", "地圖", "マップ"},
	"Calendar":               {"Calendar", "日历", "日曆", "カレンダー"},
	"Finder":                 {"Finder", "访达", "Finder", "Finder"},
	"Reveal":                 {"Reveal", "显示", "顯示", "表示"},
	"Hide":                   {"Hide", "隐藏", "隱藏", "隠す"},
	"HEX":                    {"HEX", "HEX", "HEX", "HEX"},
	"RGB":                    {"RGB", "RGB", "RGB", "RGB"},
	"HSL":                    {"HSL", "HSL", "HSL", "HSL"},
	"Local time":             {"Local time", "本地时间", "當地時間", "ローカル時刻"},
	"UTC":                    {"UTC", "UTC", "UTC", "UTC"},
	"Seconds":                {"Seconds", "秒", "秒", "秒"},
	"Milliseconds":           {"Milliseconds", "毫秒", "毫秒", "ミリ秒"},
	"Original number":        {"Original number", "原始号码", "原始號碼", "元の番号"},
	"Dialing number":         {"Dialing number", "拨号号码", "撥號號碼", "発信番号"},
	"Dialing":                {"Dialing", "拨号", "撥號", "ダイヤル"},
	"Zoom in":                {"Zoom in", "放大", "放大", "拡大"},
	"Zoom out":               {"Zoom out", "缩小", "縮小", "縮小"},
	"Fit":                    {"Fit", "适应窗口", "符合視窗", "全体表示"},
	"Map preview":            {"Map preview", "地图预览", "地圖預覽", "地図プレビュー"},
	"Map unavailable":        {"Map unavailable", "地图不可用", "地圖無法使用", "地図を表示できません"},
	"Loading map…":           {"Loading map…", "正在加载地图…", "正在載入地圖…", "地図を読み込み中…"},
	"Latitude":               {"Latitude", "纬度", "緯度", "緯度"},
	"Longitude":              {"Longitude", "经度", "經度", "経度"},
	"All day":                {"All day", "全天", "全天", "終日"},
	"January 2006":           {"January 2006", "2006 年 1 月", "2006 年 1 月", "2006年1月"},
	"Jan 2, 2006":            {"Jan 2, 2006", "2006 年 1 月 2 日", "2006 年 1 月 2 日", "2006年1月2日"},
	"Jan 2":                  {"Jan 2", "1 月 2 日", "1 月 2 日", "1月2日"},
	"Could not open content": {"Could not open content", "无法打开内容", "無法開啟內容", "内容を開けません"},
	"File unavailable":       {"File unavailable", "文件不可用", "檔案無法使用", "ファイルを利用できません"},
	"Protected location":     {"Protected location", "受保护的位置", "受保護的位置", "保護された位置情報"},
	"Preview formats leave the original unchanged": {"Preview formats leave the original unchanged", "切换预览格式不会修改原文", "切換預覽格式不會修改原文", "表示形式を変えても元の内容は変わりません"},
	"API key":          {"API key", "API 密钥", "API 金鑰", "API キー"},
	"OTP":              {"OTP", "验证码", "驗證碼", "認証コード"},
	"JWT":              {"JWT", "JWT", "JWT", "JWT"},
	"PEM":              {"PEM", "PEM", "PEM", "PEM"},
	"OpenSSH":          {"OpenSSH", "OpenSSH", "OpenSSH", "OpenSSH"},
	"GitHub":           {"GitHub", "GitHub", "GitHub", "GitHub"},
	"Slack":            {"Slack", "Slack", "Slack", "Slack"},
	"AWS":              {"AWS", "AWS", "AWS", "AWS"},
	"January":          {"January", "1 月", "1 月", "1月"},
	"February":         {"February", "2 月", "2 月", "2月"},
	"March":            {"March", "3 月", "3 月", "3月"},
	"April":            {"April", "4 月", "4 月", "4月"},
	"May":              {"May", "5 月", "5 月", "5月"},
	"June":             {"June", "6 月", "6 月", "6月"},
	"July":             {"July", "7 月", "7 月", "7月"},
	"August":           {"August", "8 月", "8 月", "8月"},
	"September":        {"September", "9 月", "9 月", "9月"},
	"October":          {"October", "10 月", "10 月", "10月"},
	"November":         {"November", "11 月", "11 月", "11月"},
	"December":         {"December", "12 月", "12 月", "12月"},
	"Sunday":           {"Sunday", "周日", "週日", "日曜日"},
	"Monday":           {"Monday", "周一", "週一", "月曜日"},
	"Tuesday":          {"Tuesday", "周二", "週二", "火曜日"},
	"Wednesday":        {"Wednesday", "周三", "週三", "水曜日"},
	"Thursday":         {"Thursday", "周四", "週四", "木曜日"},
	"Friday":           {"Friday", "周五", "週五", "金曜日"},
	"Saturday":         {"Saturday", "周六", "週六", "土曜日"},
	"Could not copy":   {"Could not copy", "复制失败", "複製失敗", "コピーできませんでした"},
	"Web page":         {"Web page", "网页", "網頁", "Web ページ"},
	"Loading page…":    {"Loading page…", "正在加载网页…", "正在載入網頁…", "ページを読み込み中…"},
	"Page unavailable": {"Page unavailable", "暂时无法加载网页", "暫時無法載入網頁", "ページを読み込めません"},
	"Retry":            {"Retry", "重试", "重試", "再試行"},
	"Open in browser":  {"Open in browser", "在浏览器打开", "在瀏覽器開啟", "ブラウザで開く"},
	"Reload page":      {"Reload page", "刷新网页", "重新整理網頁", "ページを再読み込み"},
	"Stop loading":     {"Stop loading", "停止加载", "停止載入", "読み込みを停止"},
	"Back":             {"Back", "后退", "返回", "戻る"},
	"Web preview supports HTTP and HTTPS links": {"Web preview supports HTTP and HTTPS links", "仅支持预览 HTTP 和 HTTPS 链接", "僅支援預覽 HTTP 與 HTTPS 連結", "HTTP・HTTPS リンクのみプレビューできます"},
	"Downloads open in browser":                 {"Downloads open in browser", "请在浏览器中下载", "請在瀏覽器中下載", "ダウンロードはブラウザで開いてください"},
	"Could not open browser":                    {"Could not open browser", "无法打开浏览器", "無法開啟瀏覽器", "ブラウザを開けません"},
	"Page timed out":                            {"Page timed out", "网页加载超时", "網頁載入逾時", "ページの読み込みがタイムアウトしました"},
	"Open Clipboard":                            {"Open Clipboard", "打开剪贴板", "打開剪貼簿", "クリップボードを開く"},
	"Pause Recording":                           {"Pause Recording", "暂停记录", "暫停記錄", "記録を一時停止"},
	"Resume Recording":                          {"Resume Recording", "继续记录", "繼續記錄", "記録を再開"},
	"Paste Guard":                               {"Paste Guard", "防误贴", "防誤貼", "誤貼り付け防止"},
	"Keep History":                              {"Keep History", "保留历史", "保留歷史", "履歴の保存期間"},
	"1 Day":                                     {"1 Day", "1 天", "1 天", "1 日"},
	"7 Days":                                    {"7 Days", "7 天", "7 天", "7 日"},
	"30 Days":                                   {"30 Days", "30 天", "30 天", "30 日"},
	"Forever":                                   {"Forever", "永久", "永久", "無期限"},
	"Pinned items stay":                         {"Pinned items stay", "固定记录不会过期", "釘選記錄不會到期", "ピン留めは期限なし"},
	"Retention updated":                         {"Retention updated", "保留期限已更新", "保留期限已更新", "保存期間を変更しました"},
	"Could not save retention":                  {"Could not save retention", "保留期限未保存", "保留期限未儲存", "保存期間を保存できません"},
	"Could not save history":                    {"Could not save history", "历史记录未保存", "歷史記錄未儲存", "履歴を保存できません"},
	"Could not load history":                    {"Could not load history", "历史记录读取失败", "歷史記錄讀取失敗", "履歴を読み込めません"},
	"Recording changed · Not saved":             {"Recording changed · Not saved", "记录设置未保存", "記錄設定未儲存", "記録の設定を保存できません"},
	"Paste Guard changed · Not saved":           {"Paste Guard changed · Not saved", "防误贴设置未保存", "防誤貼設定未儲存", "誤貼り付け防止を保存できません"},
	"Appearance":                                {"Appearance", "外观", "外觀", "外観"},
	"Light":                                     {"Light", "浅色", "淺色", "ライト"},
	"Dark":                                      {"Dark", "深色", "深色", "ダーク"},
	"Language":                                  {"Language", "语言", "語言", "言語"},
	"Quit Copiggy":                              {"Quit Copiggy", "退出 Copiggy", "結束 Copiggy", "Copiggy を終了"},
	"Search clipboard history":                  {"Search clipboard history", "搜索复制记录", "搜尋複製記錄", "コピー履歴を検索"},
	"Search clipboard":                          {"Search clipboard", "搜索复制内容", "搜尋複製內容", "クリップボードを検索"},
	"Search · ⌘F":                               {"Search · ⌘F", "搜索 · ⌘F", "搜尋 · ⌘F", "検索 · ⌘F"},
	"Clear or close search":                     {"Clear or close search", "清空或收起搜索", "清除或收起搜尋", "検索をクリア／閉じる"},
	"%d items":                                  {"%d items", "%d 条", "%d 筆", "%d 件"},
	"%d characters":                             {"%d characters", "%d 字符", "%d 字元", "%d 文字"},
	"⌃⌘C · Copy for this app only":              {"⌃⌘C · Copy for this app only", "⌃⌘C · 限定来源应用", "⌃⌘C · 限定來源 App", "⌃⌘C · このアプリ内のみ"},
	"Navigate":                                  {"Navigate", "切换", "切換", "移動"},
	"Preview":                                   {"Preview", "预览", "預覽", "プレビュー"},
	"Copy":                                      {"Copy", "取用", "取用", "コピー"},
	"Close":                                     {"Close", "关闭", "關閉", "閉じる"},
	"Now":                                       {"Now", "刚刚", "剛剛", "たった今"},
	"Original":                                  {"Original", "原始内容", "原始內容", "元の内容"},
	"Formatted copy":                            {"Formatted copy", "格式化副本", "格式化副本", "整形済みコピー"},
	"Current clipboard":                         {"Current clipboard", "当前剪贴板", "目前剪貼簿", "現在のクリップボード"},
	"Image · %s":                                {"Image · %s", "图片 · %s", "圖片 · %s", "画像 · %s"},
	"Source: %s":                                {"Source: %s", "来源：%s", "來源：%s", "コピー元：%s"},
	"Your copies appear here":                   {"Your copies appear here", "复制内容后，会出现在这里", "複製的內容會顯示在這裡", "コピーした内容がここに表示されます"},
	"No matches":                                {"No matches", "没有匹配记录", "沒有符合的記錄", "一致する履歴はありません"},
	"Recording paused":                          {"Recording paused", "记录已暂停", "記錄已暫停", "記録は一時停止中です"},
	"Copy for this app only":                    {"Copy for this app only", "限定来源应用", "限定來源 App", "このアプリ内のみコピー"},
	"Content preview":                           {"Content preview", "内容预览", "內容預覽", "内容のプレビュー"},
	"Clipboard changed":                         {"Clipboard changed", "剪贴板已更新", "剪貼簿已更新", "クリップボードが更新されました"},
	"Enlarge image":                             {"Enlarge image", "放大图片", "放大圖片", "画像を拡大"},
	"Close preview":                             {"Close preview", "收起预览", "收起預覽", "プレビューを閉じる"},
	"Copied image":                              {"Copied image", "复制的图片", "複製的圖片", "コピーした画像"},
	"Image unavailable":                         {"Image unavailable", "无法读取图片", "無法讀取圖片", "画像を読み込めません"},
	"Formatted draft":                           {"Formatted draft", "格式化草稿", "格式化草稿", "整形結果"},
	"Inactive":                                  {"Inactive", "未生效", "未生效", "無効"},
	"Cancel processing":                         {"Cancel processing", "取消处理", "取消處理", "処理をキャンセル"},
	"Format":                                    {"Format", "格式化", "格式化", "整形"},
	"Format JSON":                               {"Format JSON", "格式化 JSON", "格式化 JSON", "JSON を整形"},
	"Clean whitespace":                          {"Clean whitespace", "清理空白", "清除空白", "空白を整理"},
	"Original / result":                         {"Original / result", "原文 / 结果", "原文 / 結果", "元の内容／結果"},
	"Discard result":                            {"Discard result", "放弃结果", "捨棄結果", "結果を破棄"},
	"Copy result":                               {"Copy result", "取用结果", "取用結果", "結果をコピー"},
	"Copy original":                             {"Copy original", "取用原文", "取用原文", "元の内容をコピー"},
	"Close image preview":                       {"Close image preview", "关闭图片预览", "關閉圖片預覽", "画像プレビューを閉じる"},
	"Full image":                                {"Full image", "完整图片", "完整圖片", "画像全体"},
	"Only in %s":                                {"Only in %s", "仅限 %s", "僅限 %s", "%s 内のみ"},
	"Unknown app":                               {"Unknown app", "未知应用", "未知 App", "不明なアプリ"},
	"All history":                               {"All history", "全部记录", "全部記錄", "すべての履歴"},
	"Pinned":                                    {"Pinned", "已固定", "已釘選", "ピン留め"},
	"Pin":                                       {"Pin", "固定", "釘選", "ピン留め"},
	"Unpin":                                     {"Unpin", "取消固定", "取消釘選", "ピン留め解除"},
	"No pinned items":                           {"No pinned items", "暂无固定记录", "尚無釘選記錄", "ピン留めはありません"},
	"Pin a selected history item":               {"Pin a selected history item", "固定选中的历史记录", "釘選選取的歷史記錄", "選択した履歴をピン留め"},
	"Pin items to keep them here":               {"Pin items to keep them here", "固定内容，即可在这里找到", "釘選內容後即可在這裡找到", "ピン留めした内容がここに表示されます"},
	"Website icon":                              {"Website icon", "网站图标", "網站圖示", "サイトアイコン"},
	"Pin state":                                 {"Pin state", "固定状态", "釘選狀態", "ピン留め状態"},
	"Pin / Unpin · ⌘P":                          {"Pin / Unpin · ⌘P", "固定 / 取消固定 · ⌘P", "釘選 / 取消釘選 · ⌘P", "ピン留め／解除 · ⌘P"},
	"Unpinned":                                  {"Unpinned", "已取消固定", "已取消釘選", "ピン留めを解除しました"},
	"Could not save pin":                        {"Could not save pin", "固定记录保存失败", "釘選記錄儲存失敗", "ピン留めを保存できません"},
	"Could not load pins":                       {"Could not load pins", "固定记录读取失败", "釘選記錄讀取失敗", "ピン留めを読み込めません"},
	"Pin limit reached":                         {"Pin limit reached", "固定记录已达上限", "釘選記錄已達上限", "ピン留めの上限に達しました"},
	"Delete":                                    {"Delete", "删除", "刪除", "削除"},
	"Delete item · ⌘⌫":                          {"Delete item · ⌘⌫", "删除记录 · ⌘⌫", "刪除記錄 · ⌘⌫", "履歴を削除 · ⌘⌫"},
	"Deleted":                                   {"Deleted", "已删除", "已刪除", "削除しました"},
	"Could not delete item":                     {"Could not delete item", "删除失败", "刪除失敗", "削除できませんでした"},
	"Paste Guard on":                            {"Paste Guard on", "防误贴已开启", "防誤貼已開啟", "誤貼り付け防止：オン"},
	"Paste Guard off":                           {"Paste Guard off", "防误贴已关闭", "防誤貼已關閉", "誤貼り付け防止：オフ"},
	"Recording resumed":                         {"Recording resumed", "已继续记录", "已繼續記錄", "記録を再開しました"},
	"Dark appearance":                           {"Dark appearance", "已切换深色", "已切換深色", "ダークに切り替えました"},
	"Light appearance":                          {"Light appearance", "已切换浅色", "已切換淺色", "ライトに切り替えました"},
	"Allow Accessibility access":                {"Allow Accessibility access", "需要辅助功能授权", "需要輔助使用授權", "アクセシビリティの許可が必要です"},
	"Shortcut unavailable":                      {"Shortcut unavailable", "唤起快捷键不可用", "喚出快捷鍵無法使用", "ショートカットを利用できません"},
	"Could not enhance content":                 {"Could not enhance content", "增强失败，请检查内容", "增強失敗，請檢查內容", "内容を処理できませんでした"},
	"Copy the image again":                      {"Copy the image again", "图片不可用，请重新复制", "圖片無法使用，請重新複製", "画像をもう一度コピーしてください"},
	"Paste Guard is off":                        {"Paste Guard is off", "防误贴未开启", "防誤貼未開啟", "誤貼り付け防止はオフです"},
	"Could not start Paste Guard":               {"Could not start Paste Guard", "防误贴未开启，请重试", "防誤貼未開啟，請重試", "誤貼り付け防止を開始できません"},
	"Content changed · Try again":               {"Content changed · Try again", "内容已变化，请重新增强", "內容已變更，請重新增強", "内容が変わりました。再試行してください"},
	"Item unavailable · Reselect":               {"Item unavailable · Reselect", "内容已失效，请重新选择", "內容已失效，請重新選擇", "内容を選び直してください"},
	"Restart Paste Guard":                       {"Restart Paste Guard", "防误贴已停止，请重新开启", "防誤貼已停止，請重新開啟", "誤貼り付け防止を再開してください"},
	"Paste blocked":                             {"Paste blocked", "已拦截误贴", "已攔截誤貼", "貼り付けをブロックしました"},
	"Source app only":                           {"Source app only", "已限定来源应用", "已限定來源 App", "コピー元のアプリ内のみ"},
	"Protected copy failed":                     {"Protected copy failed", "限定复制未完成", "限定複製未完成", "保護コピーに失敗しました"},
	"Copied":                                    {"Copied", "已复制", "已複製", "コピーしました"},
	"Cancelled":                                 {"Cancelled", "已取消增强", "已取消增強", "キャンセルしました"},
	"Language changed":                          {"Language changed", "语言已切换", "語言已切換", "言語を変更しました"},
	"Language changed · Not saved":              {"Language changed · Not saved", "语言已切换，未保存", "語言已切換，未儲存", "言語を変更しました（保存できません）"},
	"Appearance changed · Not saved":            {"Appearance changed · Not saved", "主题已切换，未保存", "主題已切換，未儲存", "外観を変更しました（保存できません）"},
	"Clipboard changed · Try again":             {"Clipboard changed · Try again", "剪贴板已变化，请重试", "剪貼簿已變更，請重試", "内容が変わりました。再試行してください"},
	"Copying for %s":                            {"Copying for %s", "正在限定复制：%s", "正在限定複製：%s", "%s 用にコピー中"},
	"Clipboard content unavailable":             {"Clipboard content unavailable", "剪贴板内容不可用", "剪貼簿內容無法使用", "クリップボードの内容を利用できません"},
	"Empty image ignored":                       {"Empty image ignored", "空白图片已忽略", "已略過空白圖片", "空の画像を無視しました"},
	"Text exceeds 1 MiB · Not recorded":         {"Text exceeds 1 MiB · Not recorded", "文本超过 1 MiB，未记录", "文字超過 1 MiB，未記錄", "1 MiB 超のテキストは記録できません"},
	"Image exceeds 24 MP · Not recorded":        {"Image exceeds 24 MP · Not recorded", "图片超过 2400 万像素，未记录", "圖片超過 2400 萬像素，未記錄", "24 MP 超の画像は記録できません"},
	"Image data is incomplete":                  {"Image data is incomplete", "图片数据不完整", "圖片資料不完整", "画像データが不完全です"},
	"Unsupported clipboard content":             {"Unsupported clipboard content", "暂不支持此剪贴板内容", "尚不支援此剪貼簿內容", "未対応のクリップボード内容です"},
	"Content exceeds 16 MiB · Not recorded":     {"Content exceeds 16 MiB · Not recorded", "内容超过 16 MiB，未记录", "內容超過 16 MiB，未記錄", "16 MiB 超の内容は記録できません"},
}

func languageIndex(code string) int {
	for i, language := range supportedLanguages {
		if language.Code == code {
			return i
		}
	}
	return 0
}

func (s *application) currentLanguage() string {
	return supportedLanguages[languageIndex(s.language)].Code
}

func (s *application) tr(key string, args ...any) string {
	text := key
	if entry, exists := translations[key]; exists {
		text = entry[languageIndex(s.language)]
	}
	count := 0
	if len(args) == 1 {
		count, _ = args[0].(int)
	}
	if s.currentLanguage() == "en" && count == 1 {
		if key == "%d items" {
			text = "%d item"
		} else if key == "%d characters" {
			text = "%d character"
		} else if key == "%d digits" {
			text = "%d digit"
		} else if key == "%d properties" {
			text = "%d property"
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

func (s *application) setLanguage(code string) {
	if s.quitting {
		return
	}
	index := languageIndex(code)
	if supportedLanguages[index].Code != code || s.currentLanguage() == code {
		return
	}
	s.language = code
	status := "Language changed"
	s.savePreferences(func(err error) {
		if err != nil {
			s.reportNativeError("save preferences", err, "Language changed · Not saved")
		}
		s.publish()
	})
	s.reportNativeStatus(s.tr(status))
	s.publish()
}
