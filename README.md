# ainovel-cli (Bản Tiếng Việt)

Engine tự động viết tiểu thuyết dài kỳ đa tác tử (**Architect / Writer / Editor + Arbiter**) được tối ưu riêng cho quy trình **Lõi viết tiếng Trung — Giao diện & Đọc tiếng Việt**: giữ trọn sức mạnh văn phong webnovel tiếng Trung của các model Trung Quốc, đồng thời cung cấp giao diện, cầu nối đầu vào và lớp dịch mượt mà sang tiếng Việt.

> Nếu bạn muốn xem tài liệu thiết kế kiến trúc gốc bằng tiếng Trung, xem tại [docs/README.zh.md](docs/README.zh.md).

<p align="center">
  <img src="scripts/sample.gif" alt="ainovel-cli demo" width="800">
  <img src="scripts/novel.png" alt="ainovel-cli bg" width="800">
</p>

---

## Mục lục

- [1. Cơ chế "Lõi Trung — Vỏ & Đọc Việt"](#1-cơ-chế-lõi-trung--vỏ--đọc-việt)
- [2. Bảng tra cứu nhanh (Cheat Sheet)](#2-bảng-tra-cứu-nhanh-cheat-sheet)
  - [Tất cả lệnh `/` trong TUI](#tất-cả-lệnh--trong-tui)
  - [Phím tắt hay dùng](#phím-tắt-hay-dùng)
- [3. Bắt đầu nhanh & Quản lý nhiều bộ truyện](#3-bắt-đầu-nhanh--quản-lý-nhiều-bộ-truyện)
- [4. Cấu hình "Combo Vàng": Model Trung viết + Model mạnh dịch Việt](#4-cấu-hình-combo-vàng-model-trung-viết--model-mạnh-dịch-việt)
- [5. Bộ công tắc Dịch thuật & Chế độ 18+ (`/trans`)](#5-bộ-công-tắc-dịch-thuật--chế-độ-18-trans)
- [6. Đọc truyện trong TUI (`/read`) & Xuất TXT / EPUB (`/export`)](#6-đọc-truyện-trong-tui-read--xuất-txt--epub-export)
- [7. Can thiệp cốt truyện, Luật riêng & Tính năng nâng cao](#7-can-thiệp-cốt-truyện-luật-riêng--tính-năng-nâng-cao)
- [8. Cấu trúc thư mục `output/novel/`](#8-cấu-trúc-thư-mục-outputnovel)

---

## 1. Cơ chế "Lõi Trung — Vỏ & Đọc Việt"

Thay vì ép các model Trung Quốc (DeepSeek, Qwen, GLM, MiMo...) viết trực tiếp bằng tiếng Việt — vốn dễ bị sượng câu chữ và lặp cấu trúc — hệ thống tách làm 4 lớp rõ ràng:

1. **Vỏ giao diện 100% tiếng Việt**: Toàn bộ CLI, trình cài đặt ban đầu, thanh trạng thái, tiến độ, thông báo lỗi và lệnh TUI đều dùng tiếng Việt.
2. **Input Bridge tự động**: Bạn gõ yêu cầu mở truyện, thảo luận đồng sáng tạo, chỉ đạo sửa cốt truyện (Steer) hay viết luật riêng (`rules/*.md`) bằng **tiếng Việt tự nhiên**; `Arbiter` và bộ chuẩn hóa sẽ tự chuyển ngữ thành chỉ lệnh tiếng Trung chuẩn xác cho dàn viết truyện.
3. **Lõi sáng tác 100% tiếng Trung giản thể**: `Architect` (lập dàn ý/thế giới), `Writer` (viết chương), `Editor` (duyệt & đánh giá 7 tiêu chí) hoạt động hoàn toàn bằng tiếng Trung để giữ nguyên đỉnh cao văn phong gốc và khả năng cache prompt.
4. **Lớp dịch tiếng Việt độc lập (`translator`)**: Dùng riêng một model chuyên dịch (ví dụ Gemini hoặc Claude), tự xây dựng và cập nhật **từ điển nhất quán (`glossary.json`)** cho tên nhân vật, địa danh, chiêu thức và quy tắc xưng hô xuyên suốt hàng trăm chương. Bản gốc tiếng Trung không bao giờ bị ghi đè.

---

## 2. Bảng tra cứu nhanh (Cheat Sheet)

Quên lệnh khi đang dùng chỉ cần mở mục này (hoặc gõ `/help` ngay trong TUI).

### Tất cả lệnh `/` trong TUI

| Lệnh | Bí danh | Cú pháp đầy đủ | Công dụng |
| :--- | :--- | :--- | :--- |
| **`/trans`** | `/mode` | `/trans [genre=...] [tone=...] [auto=...] [--global]` | Mở bảng công tắc thể loại dịch, chế độ 18+ (`normal`/`mature`) và tự động dịch |
| **`/translate`** | `/dich` | `/translate [chương \| N-M] [from=N] [to=M] [--force]` | Dịch chương đã hoàn thành sang tiếng Việt (hỗ trợ cờ `-f` để ép dịch lại) |
| **`/read`** | `/doc` | `/read [chương]` | Mở trình đọc truyện ngay trong TUI (mặc định mở chương mới nhất) |
| **`/export`** | — | `/export [path] [lang=vi\|zh\|bilingual\|auto] [from=N] [to=M] [--overwrite]` | Xuất truyện ra `.txt` hoặc `.epub` (hỗ trợ `lang=songngu`) |
| **`/model`** | — | `/model [role]` | Đổi nóng model & `reasoning_effort` theo từng vai (`architect`, `writer`, `editor`, `translator`) |
| **`/config`** | — | `/config` | Thêm hoặc sửa Provider, API Key, Base URL, danh sách model và `context_window` |
| **`/cocreate`** | `/plan` | `/cocreate` | Tạm dừng viết để bàn bạc đồng sáng tạo và chốt hướng đi cho chặng tiếp theo |
| **`/review`** | — | `/review on\|off` | Bật/tắt chế độ dừng nghiệm thu sau từng chương hoàn thành |
| **`/next`** | — | `/next` | Mở khóa viết tiếp đúng 1 chương khi đang bật `/review on` |
| **`/start`** | — | `/start <path>` | Tạo truyện mới từ file thiết lập/đại cương có sẵn (chỉ dùng tại màn hình chào mừng) |
| **`/import`** | — | `/import <path> [--yes] [--story=open\|closed] [--continue] [--guide=<hướng dẫn>]` | Nhập tiểu thuyết sẵn có để viết tiếp (gọi `/import` không tham số để chạy tiếp lần nhập dở) |
| **`/reopen`** | — | `/reopen [hướng viết tiếp]` | Mở lại bộ truyện đã hoàn kết để viết tiếp hoặc chỉnh sửa |
| **`/simulate`** | — | `/simulate` | Đọc các file `.txt`/`.md` trong `./simulate/` để tạo/cập nhật hồ sơ văn phong mô phỏng |
| **`/importsim`** | — | `/importsim <profile.json>` | Nhập hồ sơ văn phong mô phỏng có sẵn (`simulation_profile.v1`) |
| **`/sync`** | — | `/sync [--check]` | Kiểm tra hoặc tiếp nhận các chỉnh sửa thủ công ngoài luồng trên chương gốc |
| **`/diag`** | — | `/diag` | Chẩn đoán sức khỏe bộ truyện, lỗi vòng lặp và tỉ lệ cache prompt |
| **`/help`** | — | `/help` | Mở bảng trợ giúp lệnh và phím tắt trong TUI |

### Phím tắt hay dùng

| Ngữ cảnh | Phím tắt | Tác dụng |
| :--- | :--- | :--- |
| **Chung trong TUI** | `/` | Mở danh sách gợi ý lệnh nhanh |
| | `↑` / `↓` | Di chuyển chọn lệnh trong menu gợi ý |
| | `Tab` / `Enter` | Hoàn tất lệnh gợi ý hoặc thực thi |
| | `Esc` | Đóng bảng modal hiện tại (hoặc tạm dừng engine khi đang viết) |
| | `Ctrl+R` | Bật/tắt chế độ bôi đen copy chữ (tạm tắt bắt chuột TUI rồi bấm lại để bật) |
| **Trong bảng `/trans`** | `Tab` / `Shift+Tab` / `↑` / `↓` | Chuyển qua lại giữa các dòng cấu hình |
| | `←` / `→` | Đổi giá trị của dòng đang chọn |
| | `Enter` / `Esc` | Lưu thay đổi / Hủy và đóng bảng |
| **Trong trình đọc `/read`** | `↑` `↓` hoặc `j` `k` | Cuộn từng dòng |
| | `PgUp` / `PgDn` / `Space` | Cuộn nửa trang |
| | `←` `→` hoặc `[` `]` | Chuyển chương trước / chương sau |
| | **`v`** | **Chuyển đổi tức thì Bản Tiếng Việt ↔ Nguyên bản Tiếng Trung** |
| | `r` | Tải lại nội dung chương hiện tại |
| | `Esc` hoặc `q` | Đóng trình đọc |
| **Trong Đồng sáng tạo** | `1`, `2`, `3` | Chèn nhanh gợi ý tương ứng của AI vào khung nhập |
| | `Ctrl+S` | Chốt bản phác thảo và bắt đầu chạy engine viết truyện |

---

## 3. Bắt đầu nhanh & Quản lý nhiều bộ truyện

### Tải nhanh sang máy khác (không cần cài Go)

- **Windows**: tải file `ainovel-cli_<version>_Windows_x86_64.zip` tại [Releases](https://github.com/mranex/ainovel-cli/releases/latest), giải nén lấy `ainovel-cli.exe` là chạy ngay.
- **Linux / macOS**:
  ```bash
  curl -fsSL https://raw.githubusercontent.com/mranex/ainovel-cli/main/scripts/install.sh | sh
  ```

Chạy trực tiếp từ terminal trong thư mục muốn lưu truyện:

```powershell
ainovel-cli
```

Các lệnh CLI hỗ trợ khác:

```powershell
# Kiểm tra phiên bản đang cài
ainovel-cli --version

# Tự cập nhật lên bản mới nhất (hoặc chỉ định phiên bản)
ainovel-cli update
```

### Thiết lập lần đầu

Lần đầu khởi chạy khi chưa có `~/.ainovel/config.json`, chương trình tự mở trình hướng dẫn chọn Provider -> nhập API Key -> Base URL -> Model mặc định. Sau khi xong, bạn có thể vào TUI và dùng `/config` hoặc `/model` bất cứ lúc nào để tinh chỉnh.

### Quy tắc "1 Thư mục = 1 Bộ truyện"

Mỗi bộ truyện được gắn chặt vào thư mục làm việc hiện tại (`{cwd}/output/novel/`), còn file cấu hình API `~/.ainovel/config.json` được dùng chung toàn máy:

- **Viết bộ truyện mới**: tạo thư mục mới rồi chạy bên trong:
  ```powershell
  mkdir D:\Truyen\TienHiep01
  cd D:\Truyen\TienHiep01
  ainovel-cli
  ```
- **Viết tiếp bộ truyện cũ**: chỉ cần `cd` về lại thư mục cũ và gõ `ainovel-cli`. Hệ thống tự đọc `meta/progress.json` và `meta/checkpoints.jsonl` để khôi phục chính xác từng bước đang dở mà không mất tiến độ.

### 3 cách mở truyện mới bằng tiếng Việt

Ở màn hình chào mừng khi mở thư mục trống:
1. **Khởi tạo nhanh**: Gõ trực tiếp ý tưởng bằng tiếng Việt (ví dụ: *"Viết một bộ tiên hiệp phàm nhân lưu, nam chính tên Lâm Mặc nhặt được chiếc bình cổ luyện đan, tiết tấu chắc tay, không hậu cung"*).
2. **Đồng sáng tạo (Co-create)**: Chuyển sang chế độ Đồng sáng tạo để trao đổi nhiều vòng với AI, chốt bối cảnh/nhân vật/phong cách ở cột phải rồi bấm `Ctrl+S` để bắt đầu.
3. **Từ file đại cương sẵn có**: Gõ `/start ./outline.md` để đẩy toàn bộ file thiết lập/đại cương làm yêu cầu đầu vào cho `Architect`.

---

## 4. Cấu hình "Combo Vàng": Model Trung viết + Model mạnh dịch Việt

Hệ thống tìm cấu hình theo thứ tự ưu tiên (file sau ghi đè file trước):
1. `~/.ainovel/config.json` — Cấu hình toàn cục (mọi thư mục đều dùng).
2. `./.ainovel/config.json` — Cấu hình riêng cho thư mục truyện hiện tại (nếu có).

Bạn có thể xem file mẫu chú thích đầy đủ tại [config.example.jsonc](config.example.jsonc).

Dưới đây là cấu hình gợi ý chuẩn nhất để **model Trung viết hay** mà **bản đọc tiếng Việt vẫn mượt**:
- `architect` / `writer` / `editor` -> DeepSeek / Qwen / GLM / MiMo.
- `translator` -> Gemini hoặc Claude.

```jsonc
{
  "provider": "openrouter",
  "model": "deepseek/deepseek-chat",
  "reasoning_effort": "medium",

  "providers": {
    "deepseek": {
      "api_key": "sk-deepseek-xxx",
      "base_url": "https://api.deepseek.com/v1",
      "models": [
        { "name": "deepseek-chat", "context_window": 128000 },
        { "name": "deepseek-reasoner", "context_window": 128000 }
      ]
    },
    "openrouter": {
      "api_key": "sk-or-v1-xxx",
      "base_url": "https://openrouter.ai/api/v1",
      "models": [
        { "name": "google/gemini-3.0-flash", "context_window": 200000 },
        { "name": "google/gemini-3.1-pro", "context_window": 1000000 },
        { "name": "anthropic/claude-sonnet-4-6", "context_window": 200000 }
      ]
    }
  },

  "roles": {
    "architect": { "provider": "deepseek", "model": "deepseek-reasoner", "reasoning_effort": "high" },
    "writer":    { "provider": "deepseek", "model": "deepseek-chat" },
    "editor":    { "provider": "deepseek", "model": "deepseek-chat" },
    "translator": { "provider": "openrouter", "model": "google/gemini-3.1-pro" }
  },

  "style": "default",

  "translate": {
    "genre": "auto",
    "tone": "normal",
    "auto_translate": "after_arc"
  }
}
```

### Dùng với gói OpenCode Go (`opencode-go`)

`ainovel-cli` đã tích hợp sẵn provider **`opencode-go`** (và **`opencode`** cho Pay-as-you-go):
- Tự động gắn header bắt buộc **`x-opencode-session: ses_...`** ổn định cho từng cuộc hội thoại / từng lượt agent và `User-Agent: ainovel-cli/1.0` theo đúng yêu cầu của OpenCode Go (tránh lỗi `400 MissingSessionID` và tối ưu prompt cache).
- Tự động định tuyến đúng giao thức API cho từng model trong danh mục OpenCode Go (`chat-completions` cho `deepseek-v4-pro`, `glm-5.3`, `kimi-k3.1-pro`, `mimo-v2.6-pro`; `anthropic messages` cho `qwen4-max`, `minimax-m3`, `claude-haiku-5-5`; `responses` cho `gpt-6-luna`, `grok-4.7`; `gemini` cho `gemini-4-argon`) kèm sẵn `context_window` và bảng giá.
- Hỗ trợ lấy key trực tiếp từ `"api_key"` hoặc biến môi trường `OPENCODE_API_KEY`.

Ví dụ cấu hình thuần OpenCode Go (`architect`/`writer`/`editor` chạy `deepseek-v4-pro` hoặc `glm-5.3`, còn `translator` dịch sang tiếng Việt bằng `gemini-4-argon` hoặc `claude-haiku-5-5`):

```jsonc
{
  "provider": "opencode-go",
  "model": "deepseek-v4-pro",
  "reasoning_effort": "high",

  "providers": {
    "opencode-go": {
      "api_key": "oc_sk_xxx",
      "base_url": "https://opencode.ai/inference/go",
      "models": [
        { "name": "deepseek-v4-pro" },
        { "name": "deepseek-v4.1-flash" },
        { "name": "glm-5.3" },
        { "name": "kimi-k3.1-pro" },
        { "name": "mimo-v2.6-pro" },
        { "name": "qwen4-max" },
        { "name": "minimax-m3" },
        { "name": "gpt-6-luna" },
        { "name": "gemini-4-argon" }
      ]
    }
  },

  "roles": {
    "architect":  { "provider": "opencode-go", "model": "deepseek-v4-pro", "reasoning_effort": "high" },
    "writer":     { "provider": "opencode-go", "model": "glm-5.3" },
    "editor":     { "provider": "opencode-go", "model": "deepseek-v4-pro" },
    "translator": { "provider": "opencode-go", "model": "gemini-4-argon" }
  }
}
```

### Lưu ý cấu hình quan trọng

- **`provider` là tên key (con trỏ tài khoản)** nằm trong `"providers"`, không phải tên giao thức API. Nếu dùng proxy ngoài, khai báo thêm `"type": "openai"` hoặc `"type": "anthropic"` bên trong block của provider đó.
- **Tất cả vai trò `roles.*` hỗ trợ**:
  - Sáng tác & Dịch: `architect`, `writer`, `editor`, `translator`.
  - Nhập truyện cũ (`/import`): `import_segment`, `import_analyze`, `import_synthesize` (nếu bỏ trống sẽ tự mượn model của `architect`).
- **Mức suy luận `reasoning_effort`**: `off` / `low` / `medium` / `high` / `xhigh` / `max` (bỏ trống = giữ mặc định của model).
- **Phong cách sáng tác tiếng Trung `style`**: `default` (chung), `suspense` (trinh thám/huyền nghi), `fantasy` (kỳ huyễn/tiên hiệp), `romance` (ngôn tình).
- Khi đang mở TUI, gõ `/model translator` để gán/đổi nhanh model dịch bất cứ lúc nào, hoặc `/config` để sửa key, URL và thêm model mới rồi lưu nóng.

---

## 5. Bộ công tắc Dịch thuật & Chế độ 18+ (`/trans`)

Bạn có thể đặt mặc định trong `config.json` (khối `"translate"`) và **gạt công tắc đổi nóng riêng cho từng bộ truyện** bất cứ lúc nào bằng `/trans` (hoặc `/mode`).

### Mở bảng giao diện hoặc dùng lệnh 1 dòng

- Gõ `/trans` -> mở bảng tương tác (`Tab` / `↑↓` đổi dòng, `←→` chọn giá trị, `Enter` lưu).
- Hoặc đổi ngay bằng tham số:

```text
/trans genre=tien_hiep tone=mature auto=after_commit
/trans genre=fantasy tone=normal auto=after_arc --global
```

*(Thêm `--global` hoặc chọn `"Truyện này + Mặc định"` ở dòng Phạm vi lưu để ghi đè luôn cả mặc định trong `config.json`; mặc định chỉ lưu riêng cho cuốn truyện hiện tại trong `output/novel/meta/run.json`.)*

### Ý nghĩa 3 công tắc

| Công tắc | Các giá trị | Tác dụng chi tiết |
| :--- | :--- | :--- |
| **`genre`**<br>*(Thể loại dịch)* | `auto`<br>`tien_hiep`<br>`co_trang`<br>`fantasy`<br>`hien_dai` | - `auto`: tự phân tích nền truyện để chọn văn phong.<br>- `tien_hiep`: văn phong tu tiên/huyền huyễn, chuẩn Hán-Việt cảnh giới & tông môn.<br>- `co_trang`: giang hồ, kiếm hiệp, triều đình.<br>- `fantasy`: **khử Hán-Việt cho tên riêng phương Tây**, giữ nguyên tên gốc Latinh (Arthur, Eldoria...) và xưng hô gọn tự nhiên.<br>- `hien_dai`: đô thị, đời thường, lời thoại tự nhiên không cổ trang. |
| **`tone`**<br>*(Sắc thái / 18+)* | `normal`<br>`mature` | - `normal`: chuẩn phổ thông.<br>- `mature`: chế độ **Trưởng thành 18+** — truyền giữ trọn vẹn ý đồ nhạy cảm/bạo lực/tình cảm qua **Input Bridge** cho Writer, đồng thời dịch sát độ nóng nguyên tác sang tiếng Việt, không tự kiểm duyệt hay làm nhẹ chữ. |
| **`auto`**<br>*(Tự động dịch)* | `after_arc`<br>`after_commit`<br>`manual` | - `after_arc` *(khuyên dùng)*: đợi `Editor` duyệt xong cả Arc/Quyển hoàn chỉnh (đã viết lại/trau chuốt xong) rồi mới dịch, tránh dịch nhầm bản nháp bị sửa và tiết kiệm chi phí.<br>- `after_commit`: vừa viết xong chương nào dịch ngay chương đó để bạn đọc nóng.<br>- `manual`: tắt tự động, chỉ dịch khi bạn gọi `/translate`. |

### Dịch thủ công & Kiểm soát từ điển tên riêng (`/translate` / `/dich`)

- `/translate` (hoặc `/dich`): tự tìm và dịch tất cả chương đã hoàn thành chưa có bản Việt (hoặc bản gốc vừa bị viết lại).
- `/translate 5`: dịch riêng chương 5.
- `/translate 1-10` (hoặc `from=1 to=10`): dịch khoảng từ chương 1 đến chương 10.
- `/translate 1-10 --force` (hoặc `-f`): ép dịch lại dù đã có bản Việt (hữu ích khi bạn vừa đổi `genre=fantasy` hoặc `tone=mature`).

**Mẹo chỉnh tên riêng cố định**: Toàn bộ bảng tên nhân vật, địa danh, chiêu thức (`entries`) và quy tắc xưng hô (`pronoun_guide`) được lưu tại:
`output/novel/translations/vi/glossary.json`.
Nếu không ưng cách dịch tên một nhân vật nào đó, bạn có thể sửa trực tiếp trong `glossary.json` rồi chạy `/translate <chương> --force` — các chương sau sẽ tự động theo tên mới bạn đã chốt.

---

## 6. Đọc truyện trong TUI (`/read`) & Xuất TXT / EPUB (`/export`)

### Đọc ngay trong cửa sổ terminal

```text
/read          # Mở chương đã hoàn thành mới nhất
/read 12       # Mở thẳng chương 12
```

- Thanh bên (Sidebar) luôn có mục **Dịch Việt** báo tiến độ `đã dịch / tổng chương xong`, số chương bị cũ (`cũ`), hoặc chương đang dịch ngầm.
- Khi đang đọc bản tiếng Việt, chỉ cần bấm phím **`v`** là chuyển tức thì sang **nguyên bản tiếng Trung** của đúng chương đó để đối chiếu văn phong (bấm `v` lần nữa để quay lại tiếng Việt).
- Nếu nguyên bản tiếng Trung vừa bị Engine/Editor viết lại mà chưa kịp dịch lại, trình đọc sẽ hiện cảnh báo nhắc chạy `/translate <chương>`.

### Xuất file TXT / EPUB (`/export`)

Đuôi file quyết định định dạng (`.txt` hoặc `.epub`). Nếu bỏ trống đường dẫn, mặc định xuất `.txt` ngay vào thư mục truyện:

```text
/export                                      # Xuất mặc định (ưu tiên Tiếng Việt nếu đã dịch)
/export lang=vi                              # Ép xuất bản Tiếng Việt (.txt)
/export ./TruyenCuaTao.epub lang=vi          # Xuất EPUB 3 Tiếng Việt đọc trên điện thoại/Kindle
/export ./BanGocTrung.epub lang=zh           # Xuất EPUB nguyên bản Tiếng Trung
/export ./SongNgu.epub lang=bilingual        # Xuất EPUB Song ngữ (mỗi đoạn Việt kèm Trung ngay dưới)
/export lang=songngu from=1 to=30 --overwrite
```

> **Lưu ý**: Nếu chọn `lang=vi` hoặc `auto` mà có vài chương ở cuối chưa kịp dịch, trình xuất vẫn tạo file thành công, giữ tạm nguyên bản tiếng Trung cho các chương đó và thông báo rõ danh sách số chương chưa dịch.

---

## 7. Can thiệp cốt truyện, Luật riêng & Tính năng nâng cao

### Can thiệp thời gian thực (Steer) bằng tiếng Việt

Trong lúc AI đang chạy tự động, không cần dừng máy: cứ gõ thẳng chỉ đạo tiếng Việt vào ô nhập bên dưới và nhấn `Enter`:
- *"Đổi tính cách nam chính bớt hiền lại, hành động quyết đoán và tàn nhẫn hơn với kẻ thù."*
- *"Đẩy tuyến tình cảm lên sớm ở chương 8."*
- *"Viết tới hết chương 20 thì dừng lại chờ tao đọc."*

`Arbiter` sẽ tự phân loại: hỏi đáp thì trả lời ngay, sửa cốt truyện thì xếp hàng cho `Architect` hoặc `Editor` sửa an toàn ở ranh giới chương.

### Tự đặt Luật viết riêng (`rules/*.md`) bằng tiếng Việt

Không cần sửa code hay học cú pháp phức tạp: tạo file `.md` bất kỳ trong:
- `~/.ainovel/rules/` — áp dụng cho **mọi bộ truyện** của bạn.
- `./.ainovel/rules/` — áp dụng **riêng cho bộ truyện** ở thư mục hiện tại.

Viết bằng văn xuôi tiếng Việt bình thường, ví dụ:
- *"Mỗi chương viết khoảng 2500 đến 3200 chữ."*
- *"Không dùng văn phong sáo rỗng hay giảng đạo lý dài dòng."*
- *"Tuyệt đối không để nhân vật chính tha mạng cho kẻ từng muốn giết mình."*

Khi bắt đầu hoặc sửa rule, Input Bridge sẽ chuẩn hóa chúng thành ràng buộc số chữ/từ cấm và ghi vào `meta/user_rules.json` cho toàn đội viết truyện tuân theo.

### Các quy trình nâng cao khác

- **Nghiệm thu từng chương**: Gõ `/review on` để Engine dừng chờ sau mỗi chương viết mới. Đọc xong bấm `/next` để viết tiếp đúng 1 chương nữa; gõ `/review off` để quay lại chạy tự động liên tục.
- **Sửa tay trực tiếp chương gốc**: Nếu bạn tự mở và sửa các file chương tiếng Trung trong `output/novel/chapters/*.md`, gõ `/sync --check` để kiểm tra thay đổi và `/sync` để AI cập nhật lại tóm tắt, timeline và phục bút.
- **Học văn phong từ truyện mẫu (`/simulate`)**: Tạo thư mục `./simulate/` ngay tại thư mục truyện, thả vài chương mẫu (`.txt` hoặc `.md`) vào rồi chạy `/simulate`. Hệ thống sẽ phân tích nhịp điệu, cấu trúc mở/kết chương để làm hồ sơ tham khảo (hoặc nạp file hồ sơ có sẵn bằng `/importsim ./profile.json`).
- **Nhập truyện cũ có sẵn để viết tiếp (`/import`)**: Chuẩn bị file `.txt`/`.md` ở thư mục trống mới rồi chạy `/import ./truyen_cu.txt`. Hỗ trợ `--yes` (tự xác nhận chia chương), `--story=open|closed`, `--continue` (nhập xong viết tiếp luôn).
- **Mở lại truyện đã hoàn kết (`/reopen`)**: Khi truyện đã ở trạng thái hoàn thành, dùng `/reopen [hướng viết ngoại truyện / phần tiếp]` để tiếp tục.
- **Chẩn đoán & Sửa lỗi (`/diag`)**: Sinh báo cáo sức khỏe tiến độ, lỗi kẹt vòng lặp, tỉ lệ cache prompt (`meta/diag-export.md`).
- **Chạy ngầm không giao diện (Headless)** — dùng cho máy chủ hoặc chạy treo máy sau khi đã thiết lập xong `config.json` bằng TUI ít nhất 1 lần:
  ```powershell
  # Bắt đầu bộ truyện mới bằng 1 câu tiếng Việt
  ainovel-cli --headless --prompt "Viết một bộ đô thị dị năng lấy bối cảnh hiện đại"

  # Đọc yêu cầu dài từ file
  ainovel-cli --headless --prompt-file yeu_cau.txt

  # Tiếp tục chạy ngầm bộ truyện dang dở trong thư mục hiện tại
  ainovel-cli --headless
  ```
  *(Lưu ý: không truyền thẳng chuỗi yêu cầu trần thiếu cờ `--prompt` hay `--prompt-file` trên dòng lệnh.)*

---

## 8. Cấu trúc thư mục `output/novel/`

Mọi dữ liệu của bộ truyện trong thư mục hiện tại đều nằm gọn tại `output/novel/` (ghi file an toàn qua temp + fsync + rename nên tắt ngang hay mất điện cũng không hỏng dữ liệu):

```text
output/novel/
├── book.md                 # Tên truyện & giới thiệu (tiếng Trung)
├── chapters/               # Chương hoàn chỉnh gốc tiếng Trung (01.md, 02.md...)
├── drafts/                 # Bản nháp đang viết dở
├── summaries/              # Tóm tắt từng chương (JSON)
├── reviews/                # Báo cáo đánh giá của Editor
├── premise.md              # Tiền đề cốt truyện
├── outline.json            # Dàn ý các chương đã triển khai
├── layered_outline.json    # Dàn ý phân tầng Quyển / Arc
├── characters.json         # Hồ sơ nhân vật
├── world_rules.json        # Quy tắc thế giới quan
├── timeline.jsonl / .md    # Dòng thời gian sự kiện
├── translations/
│   └── vi/                 # Tầng dịch tiếng Việt (tách biệt hoàn toàn bản gốc)
│       ├── book.md         # Tên truyện & giới thiệu tiếng Việt
│       ├── glossary.json   # Từ điển tên riêng, thuật ngữ & quy tắc xưng hô
│       ├── manifest.json   # Chỉ mục theo dõi bản dịch & phát hiện chương cũ (SHA-256)
│       └── chapters/       # Bản dịch tiếng Việt từng chương (01.md, 02.md...)
└── meta/
    ├── book.json           # Dữ liệu gốc tên truyện / trạng thái quyển
    ├── run.json            # Cấu hình chạy riêng của cuốn truyện (gồm công tắc /trans)
    ├── progress.json       # Tiến độ hiện tại
    ├── compass.json        # La bàn định hướng kết cục dài kỳ
    ├── foreshadow.json     # Sổ theo dõi phục bút
    ├── checkpoints.jsonl   # Điểm khôi phục từng bước công cụ
    └── ...
```

---

**Giấy phép**: MIT — Bản nâng cấp Việt hóa giao diện, Input Bridge & Lớp dịch đa thể loại dựa trên [voocel/ainovel-cli](https://github.com/voocel/ainovel-cli).
