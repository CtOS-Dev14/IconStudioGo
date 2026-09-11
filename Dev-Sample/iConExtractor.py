import ctypes
import io
import os
import struct
import sys
import threading
import tkinter as tk
from tkinter import filedialog, messagebox, ttk
from PIL import Image, ImageTk
import pefile

APP_NAME = "Icon Studio"
APP_VERSION = "v1.5.2"
APP_BUILD = "1.5.2.0"
APP_DESC = "Windows UHD Multi-size Master Pack Icon Studio"

try:
    from tkinterdnd2 import DND_FILES, TkinterDnD
    HAS_DND = True
except ImportError:
    HAS_DND = False

RT_ICON = 3
RT_GROUP_ICON = 14

def get_bundle_path(relative_path):
    """PyInstaller 번들 내부 또는 로컬 경로를 반환"""
    if hasattr(sys, '_MEIPASS'):
        return os.path.join(sys._MEIPASS, relative_path)
    return os.path.join(os.path.abspath("."), relative_path)

class DisableFileSystemRedirection:
    _disable = ctypes.windll.kernel32.Wow64DisableWow64FsRedirection
    _revert = ctypes.windll.kernel32.Wow64RevertWow64FsRedirection

    def __enter__(self):
        self.old_val = ctypes.c_void_p()
        self._disable(ctypes.byref(self.old_val))

    def __exit__(self, type, value, traceback):
        self._revert(self.old_val)

def decode_icon_layer_to_image(raw_bytes, entry):
    if not raw_bytes:
        return None

    if raw_bytes.startswith(b'\x89PNG\r\n\x1a\n'):
        try:
            return Image.open(io.BytesIO(raw_bytes)).convert("RGBA")
        except Exception:
            pass

    try:
        ico_tmp = bytearray(struct.pack("<HHH", 0, 1, 1))
        ico_tmp.extend(struct.pack(
            "<BBBBHHII",
            entry["raw_w"], entry["raw_h"], entry["colors"], 0,
            entry["planes"], entry["bit_count"], len(raw_bytes), 22
        ))
        ico_tmp.extend(raw_bytes)
        img = Image.open(io.BytesIO(ico_tmp)).convert("RGBA")

        alpha = img.getchannel('A')
        if alpha.getextrema() == (0, 0):
            img.putalpha(255)
        return img
    except Exception:
        pass

    try:
        if len(raw_bytes) >= 40:
            w, h, planes, bpp, comp, img_sz, _, _, clr_used, _ = struct.unpack("<iiHHIIiiII", raw_bytes[4:40])
            real_h = abs(h) // 2
            clr_count = (1 << bpp) if (clr_used == 0 and bpp <= 8) else clr_used
            offset_to_bits = 14 + 40 + (clr_count * 4)

            bmp_header = struct.pack("<2sIHHI", b"BM", 14 + len(raw_bytes), 0, 0, offset_to_bits)
            new_info_header = struct.pack("<iiiHHIIiiII", 40, w, real_h, planes, bpp, comp, img_sz, 0, 0, clr_used, 0)
            bmp_data = bmp_header + new_info_header + raw_bytes[40:]
            return Image.open(io.BytesIO(bmp_data)).convert("RGBA")
    except Exception:
        pass

    w = entry.get("width", 16)
    h = entry.get("height", 16)
    return Image.new("RGBA", (w, h), (0, 0, 0, 0))

class ModernIconStudio:
    def __init__(self, root):
        self.root = root
        self.root.title(f"{APP_NAME} {APP_VERSION}")
        self.root.configure(bg="#0F172A")

        self._center_window(1060, 700)
        self.root.minsize(820, 520)

        self._load_and_apply_app_icons()

        self.current_pe = None
        self.file_path = None
        self.raw_icon_cache = {}
        self.group_cache = {}
        self.tile_widgets = {}
        self.selected_grp_ids = set()
        self.active_preview_id = None
        self.tk_cache = []
        self._last_cols = 0

        self._init_styles()
        self._build_ui()
        self._setup_dnd()

    def _center_window(self, width, height):
        self.root.update_idletasks()
        screen_width = self.root.winfo_screenwidth()
        screen_height = self.root.winfo_screenheight()
        x = (screen_width - width) // 2
        y = (screen_height - height) // 2
        self.root.geometry(f"{width}x{height}+{x}+{y}")

    def _load_and_apply_app_icons(self):
        """app_icon.ico 파일에서 16, 24, 32, 48, 64, 128, 256 전 규격을 추출해 등록"""
        self.tk_icons = []
        self.master_about_img = None
        ico_path = get_bundle_path("app_icon.ico")

        if os.path.exists(ico_path):
            try:
                # 윈도우 네이티브 타이틀바/작업표시줄에 ico 파일 직접 바인딩
                self.root.iconbitmap(default=ico_path)
                
                # 내부 PhotoImage로도 모든 레이어 등록
                with open(ico_path, "rb") as f:
                    data = f.read()
                if len(data) > 6:
                    _, _, count = struct.unpack("<HHH", data[:6])
                    offset = 6
                    for _ in range(count):
                        w, h = data[offset], data[offset+1]
                        real_sz = 256 if w == 0 else w
                        sz = struct.unpack("<I", data[offset+8:offset+12])[0]
                        pos = struct.unpack("<I", data[offset+12:offset+16])[0]
                        offset += 16

                        chunk = data[pos:pos+sz]
                        if chunk.startswith(b'\x89PNG\r\n\x1a\n'):
                            img = Image.open(io.BytesIO(chunk)).convert("RGBA")
                            self.tk_icons.append(ImageTk.PhotoImage(img))
                            if real_sz == 64:
                                self.master_about_img = img

                if self.tk_icons:
                    self.root.iconphoto(True, *self.tk_icons)
            except Exception:
                pass

        if not self.master_about_img:
            self.master_about_img = Image.new("RGBA", (64, 64), (79, 70, 229, 255))

    def _init_styles(self):
        style = ttk.Style()
        style.theme_use('clam')
        style.configure("Slim.Horizontal.TProgressbar", foreground='#6366F1', background='#6366F1', thickness=3)

        style.configure(
            "Modern.Vertical.TScrollbar",
            gripcount=0, background="#CBD5E1", darkcolor="#CBD5E1", lightcolor="#CBD5E1",
            troughcolor="#F8FAFC", bordercolor="#F8FAFC", arrowcolor="#64748B",
            arrowsize=11, relief="flat"
        )
        style.map("Modern.Vertical.TScrollbar", background=[('pressed', '#6366F1'), ('active', '#94A3B8')])

        style.configure(
            "Modern.Treeview",
            background="#F8FAFC", foreground="#1E293B", fieldbackground="#F8FAFC",
            font=("Segoe UI", 9), rowheight=26, borderwidth=0
        )
        style.map("Modern.Treeview", background=[('selected', '#EEF2FF')], foreground=[('selected', '#4F46E5')])

        style.configure(
            "Modern.Treeview.Heading",
            background="#1E293B", foreground="#F8FAFC",
            font=("Malgun Gothic", 9, "bold"), relief="flat", padding=(6, 6)
        )
        style.map("Modern.Treeview.Heading", background=[('active', '#334155')])

    def _set_btn_state(self, btn, enabled, active_bg, active_fg, is_bold=False):
        font_weight = "bold" if is_bold else "normal"
        if enabled:
            btn.config(state=tk.NORMAL, bg=active_bg, fg=active_fg, cursor="hand2", font=("Malgun Gothic", 9, font_weight))
        else:
            btn.config(state=tk.DISABLED, bg="#E2E8F0", disabledforeground="#94A3B8", fg="#94A3B8", cursor="arrow", font=("Malgun Gothic", 9, font_weight))

    def _build_ui(self):
        header = tk.Frame(self.root, bg="#0F172A", height=48, padx=16)
        header.pack(fill=tk.X)
        header.pack_propagate(False)

        title_frame = tk.Frame(header, bg="#0F172A")
        title_frame.pack(side=tk.LEFT, pady=10)

        self.lbl_title = tk.Label(title_frame, text=APP_NAME, font=("Segoe UI", 12, "bold"), fg="#F8FAFC", bg="#0F172A")
        self.lbl_title.pack(side=tk.LEFT)

        ver_badge = tk.Label(
            title_frame, text=APP_VERSION, font=("Segoe UI", 8, "bold"),
            fg="#818CF8", bg="#1E293B", padx=6, pady=1
        )
        ver_badge.pack(side=tk.LEFT, padx=(8, 0))

        self.lbl_count = tk.Label(header, text="파일을 열거나 창 위로 끌어다 놓으세요", font=("Malgun Gothic", 9), fg="#94A3B8", bg="#0F172A")
        self.lbl_count.pack(side=tk.LEFT, padx=14, pady=11)

        self.progress_bar = ttk.Progressbar(header, style="Slim.Horizontal.TProgressbar", mode='indeterminate', length=100)

        btn_info = tk.Button(
            header, text="ⓘ 정보", font=("Malgun Gothic", 8),
            bg="#1E293B", fg="#94A3B8", activebackground="#334155", activeforeground="white",
            relief=tk.FLAT, padx=8, pady=4, cursor="hand2", command=self.show_about_dialog
        )
        btn_info.pack(side=tk.RIGHT, padx=(6, 0), pady=9)

        btn_open = tk.Button(
            header, text="파일 열기", font=("Malgun Gothic", 9, "bold"),
            bg="#4F46E5", fg="white", activebackground="#4338CA", activeforeground="white",
            relief=tk.FLAT, padx=14, pady=4, cursor="hand2", command=self.open_file_dialog
        )
        btn_open.pack(side=tk.RIGHT, pady=9)

        self.btn_select_all = tk.Button(
            header, text="전체 선택", font=("Malgun Gothic", 9),
            bg="#1E293B", fg="#E2E8F0", activebackground="#334155", activeforeground="white",
            relief=tk.FLAT, padx=10, pady=4, cursor="hand2", command=self.toggle_select_all
        )
        self.btn_select_all.pack(side=tk.RIGHT, padx=6, pady=9)
        self._set_btn_state(self.btn_select_all, False, "#1E293B", "#E2E8F0")

        main_body = tk.Frame(self.root, bg="#F1F5F9")
        main_body.pack(fill=tk.BOTH, expand=True)

        self.canvas_box = tk.Frame(main_body, bg="#F8FAFC")
        self.canvas_box.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)

        self.canvas = tk.Canvas(self.canvas_box, bg="#F8FAFC", highlightthickness=0, takefocus=1)
        self.scrollbar = ttk.Scrollbar(self.canvas_box, orient=tk.VERTICAL, command=self.canvas.yview, style="Modern.Vertical.TScrollbar")
        self.grid_frame = tk.Frame(self.canvas, bg="#F8FAFC")

        self.grid_frame.bind("<Configure>", self._on_frame_configure)
        self.canvas_window = self.canvas.create_window((0, 0), window=self.grid_frame, anchor="nw")
        self.canvas.configure(yscrollcommand=self.scrollbar.set)

        self.canvas.bind('<Configure>', self._on_canvas_resize)
        self.canvas.bind("<Enter>", lambda e: self.root.bind_all("<MouseWheel>", self._on_mousewheel))
        self.canvas.bind("<Leave>", lambda e: self.root.unbind_all("<MouseWheel>"))

        self.canvas.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)
        self.scrollbar.pack(side=tk.RIGHT, fill=tk.Y)

        self.right_panel = tk.Frame(main_body, bg="#FFFFFF", width=270, bd=1, relief=tk.SOLID)
        self.right_panel.pack(side=tk.RIGHT, fill=tk.Y)
        self.right_panel.pack_propagate(False)

        inspect_header_box = tk.Frame(self.right_panel, bg="#1E293B", height=38, padx=12)
        inspect_header_box.pack(fill=tk.X)
        inspect_header_box.pack_propagate(False)

        tk.Label(
            inspect_header_box, text="아이콘 인스펙터", font=("Malgun Gothic", 9, "bold"),
            fg="#F8FAFC", bg="#1E293B"
        ).pack(side=tk.LEFT, pady=8)

        self.preview_box = tk.Frame(self.right_panel, bg="#F8FAFC", height=130, bd=1, relief=tk.SOLID)
        self.preview_box.pack(fill=tk.X, padx=14, pady=(12, 10))
        self.preview_box.pack_propagate(False)

        self.lbl_large_preview = tk.Label(self.preview_box, bg="#F8FAFC")
        self.lbl_large_preview.place(relx=0.5, rely=0.5, anchor=tk.CENTER)

        self.lbl_inspect_title = tk.Label(
            self.right_panel, text="선택 없음", font=("Segoe UI", 10, "bold"),
            fg="#0F172A", bg="#FFFFFF"
        )
        self.lbl_inspect_title.pack(anchor="w", padx=14, pady=(0, 8))

        tree_frame = tk.Frame(self.right_panel, bg="#FFFFFF")
        tree_frame.pack(fill=tk.BOTH, expand=True, padx=14, pady=(0, 10))

        self.layer_tree = ttk.Treeview(
            tree_frame, columns=("size", "bpp"), show="headings",
            style="Modern.Treeview"
        )
        self.layer_tree.heading("size", text="해상도 규격")
        self.layer_tree.heading("bpp", text="색상 깊이")
        self.layer_tree.column("size", width=130, anchor="center")
        self.layer_tree.column("bpp", width=95, anchor="center")
        self.layer_tree.pack(fill=tk.BOTH, expand=True)

        single_action_frame = tk.Frame(self.right_panel, bg="#FFFFFF", padx=14, pady=8)
        single_action_frame.pack(fill=tk.X)

        self.btn_extract_one_ico = tk.Button(
            single_action_frame, text="UHD 256px ICO 팩 저장", relief=tk.FLAT, pady=6,
            command=lambda: self.extract_single("ico")
        )
        self.btn_extract_one_ico.pack(fill=tk.X, pady=(0, 4))
        self._set_btn_state(self.btn_extract_one_ico, False, "#EEF2FF", "#4F46E5", is_bold=True)

        self.btn_extract_one_png = tk.Button(
            single_action_frame, text="최고화질 투명 PNG 저장", relief=tk.FLAT, pady=5,
            command=lambda: self.extract_single("png")
        )
        self.btn_extract_one_png.pack(fill=tk.X)
        self._set_btn_state(self.btn_extract_one_png, False, "#F8FAFC", "#475569")

        bottom_bar = tk.Frame(self.root, bg="#FFFFFF", height=48, padx=14, bd=1, relief=tk.SOLID)
        bottom_bar.pack(fill=tk.X, side=tk.BOTTOM)
        bottom_bar.pack_propagate(False)

        self.lbl_selected_status = tk.Label(bottom_bar, text="0개 선택됨", font=("Malgun Gothic", 9), fg="#64748B", bg="#FFFFFF")
        self.lbl_selected_status.pack(side=tk.LEFT, pady=13)

        self.btn_batch_png = tk.Button(
            bottom_bar, text="선택 항목 고화질 PNG 일괄 추출", relief=tk.FLAT, padx=12, pady=5,
            command=lambda: self.batch_extract("png")
        )
        self.btn_batch_png.pack(side=tk.RIGHT, pady=8, padx=(4, 0))
        self._set_btn_state(self.btn_batch_png, False, "#F1F5F9", "#334155")

        self.btn_batch_ico = tk.Button(
            bottom_bar, text="선택 항목 UHD ICO 팩 일괄 추출", relief=tk.FLAT, padx=14, pady=5,
            command=lambda: self.batch_extract("ico")
        )
        self.btn_batch_ico.pack(side=tk.RIGHT, pady=8, padx=(0, 4))
        self._set_btn_state(self.btn_batch_ico, False, "#4F46E5", "white", is_bold=True)

    def show_about_dialog(self):
        dlg = tk.Toplevel(self.root)
        dlg.title("프로그램 정보")
        dlg.geometry("380x290")
        dlg.resizable(False, False)
        dlg.configure(bg="#FFFFFF")
        dlg.transient(self.root)
        dlg.grab_set()

        x = self.root.winfo_x() + (self.root.winfo_width() - 380) // 2
        y = self.root.winfo_y() + (self.root.winfo_height() - 290) // 2
        dlg.geometry(f"+{x}+{y}")

        about_tk = ImageTk.PhotoImage(self.master_about_img)
        icon_lbl = tk.Label(dlg, image=about_tk, bg="#FFFFFF")
        icon_lbl.image = about_tk
        icon_lbl.pack(pady=(18, 6))

        tk.Label(dlg, text=APP_NAME, font=("Segoe UI", 13, "bold"), fg="#0F172A", bg="#FFFFFF").pack()
        tk.Label(dlg, text=f"버전: {APP_VERSION} (빌드 {APP_BUILD})", font=("Segoe UI", 9), fg="#6366F1", bg="#FFFFFF").pack(pady=(2, 6))

        tk.Label(
            dlg,
            text=f"{APP_DESC}\n\n• 7종 멀티레이어 마스터 아이콘 팩 내장\n• 16px부터 256px까지 전 규격 누락 없는 완전 복원\n• Windows 10/11 바로가기 고화질 보정 지원",
            font=("Malgun Gothic", 8), fg="#475569", bg="#FFFFFF", justify=tk.CENTER
        ).pack(padx=20)

        tk.Button(
            dlg, text="닫기", font=("Malgun Gothic", 9),
            bg="#F1F5F9", fg="#334155", relief=tk.FLAT, padx=16, pady=4, cursor="hand2",
            command=dlg.destroy
        ).pack(side=tk.BOTTOM, pady=16)

    def _setup_dnd(self):
        if not HAS_DND:
            return

        def drop_handler(event):
            raw_data = event.data
            if raw_data.startswith('{') and raw_data.endswith('}'):
                file_path = raw_data[1:-1]
            else:
                file_path = raw_data.split()[0]

            if os.path.isfile(file_path):
                self.load_target_file(file_path)

        self.root.drop_target_register(DND_FILES)
        self.root.dnd_bind('<<Drop>>', drop_handler)

    def _on_frame_configure(self, event=None):
        self.canvas.update_idletasks()
        bbox = self.canvas.bbox("all")
        if not bbox:
            return
        self.canvas.configure(scrollregion=(0, 0, bbox[2], max(bbox[3], self.canvas.winfo_height())))

    def _on_canvas_resize(self, event):
        self.canvas.itemconfig(self.canvas_window, width=event.width)
        if not self.group_cache:
            return

        avail_width = max(100, event.width - 20)
        cols = max(3, avail_width // 96)

        if cols != self._last_cols:
            self._last_cols = cols
            self._relayout_grid(cols)
        else:
            self._on_frame_configure()

    def _relayout_grid(self, cols):
        for i in range(cols):
            self.grid_frame.grid_columnconfigure(i, weight=1)
        for i in range(cols, 25):
            self.grid_frame.grid_columnconfigure(i, weight=0)

        r, c = 0, 0
        for grp_id in self.group_cache.keys():
            widget = self.tile_widgets.get(grp_id)
            if widget:
                widget["card"].grid(row=r, column=c, padx=5, pady=5)
                c += 1
                if c >= cols:
                    c = 0
                    r += 1

        self.grid_frame.update_idletasks()
        self._on_frame_configure()

    def _on_mousewheel(self, event):
        bbox = self.canvas.bbox("all")
        if not bbox:
            return
        total_h = bbox[3] - bbox[1]
        canvas_h = self.canvas.winfo_height()
        if total_h <= canvas_h:
            return
        steps = int(-1 * (event.delta / 120))
        self.canvas.yview_scroll(steps, "units")

    def open_file_dialog(self):
        fpath = filedialog.askopenfilename(
            filetypes=[("바이너리 및 DLL", "*.exe;*.dll;*.mun;*.scr;*.ocx;*.cpl"), ("모든 파일", "*.*")]
        )
        if fpath:
            self.load_target_file(fpath)

    def load_target_file(self, fpath):
        self.file_path = fpath
        self.canvas.yview_moveto(0.0)

        for w in self.grid_frame.winfo_children():
            w.destroy()

        self.tile_widgets.clear()
        self.group_cache.clear()
        self.raw_icon_cache.clear()
        self.selected_grp_ids.clear()
        self.active_preview_id = None
        self.tk_cache.clear()

        self.progress_bar.pack(side=tk.LEFT, padx=8, pady=11)
        self.progress_bar.start(10)
        self.lbl_count.config(text=f"{os.path.basename(fpath)} 분석 중...")

        threading.Thread(target=self._parse_worker, args=(fpath,), daemon=True).start()

    def _resolve_real_resource_file(self, fpath):
        real_path = os.path.expandvars(os.path.expanduser(fpath))
        filename = os.path.basename(real_path).lower()
        sys_root = os.environ.get("SystemRoot", "C:\\Windows")

        if filename.endswith(".mun"):
            return real_path

        mun_candidate = os.path.join(sys_root, "SystemResources", f"{filename}.mun")
        if os.path.exists(mun_candidate):
            return mun_candidate

        if "system32" in real_path.lower() or "syswow64" in real_path.lower():
            mun_candidate = os.path.join(sys_root, "SystemResources", f"{filename}.mun")
            if os.path.exists(mun_candidate):
                return mun_candidate

        return real_path

    def _extract_resource_data(self, pe, entry):
        if hasattr(entry, 'data') and hasattr(entry.data, 'struct'):
            return pe.get_data(entry.data.struct.OffsetToData, entry.data.struct.Size)
        if hasattr(entry, 'directory') and entry.directory.entries:
            sub = entry.directory.entries[0]
            if hasattr(sub, 'data') and hasattr(sub.data, 'struct'):
                return pe.get_data(sub.data.struct.OffsetToData, sub.data.struct.Size)
            elif hasattr(sub, 'directory') and sub.directory.entries:
                sub2 = sub.directory.entries[0]
                if hasattr(sub2, 'data') and hasattr(sub2.data, 'struct'):
                    return pe.get_data(sub2.data.struct.OffsetToData, sub2.data.struct.Size)
        return None

    def _parse_worker(self, fpath):
        try:
            target_path = self._resolve_real_resource_file(fpath)
            with DisableFileSystemRedirection():
                with open(target_path, "rb") as f:
                    file_bytes = f.read()

            pe = pefile.PE(data=file_bytes, fast_load=True)
            pe.parse_data_directories(directories=[
                pefile.DIRECTORY_ENTRY['IMAGE_DIRECTORY_ENTRY_RESOURCE']
            ])

            if not hasattr(pe, "DIRECTORY_ENTRY_RESOURCE"):
                self.root.after(0, lambda: self._on_parse_complete({}, {}, "아이콘 리소스 없음"))
                return

            rt_icons = next((e for e in pe.DIRECTORY_ENTRY_RESOURCE.entries if e.id == RT_ICON), None)
            rt_groups = next((e for e in pe.DIRECTORY_ENTRY_RESOURCE.entries if e.id == RT_GROUP_ICON), None)

            if not rt_groups or not rt_icons:
                self.root.after(0, lambda: self._on_parse_complete({}, {}, "아이콘 그룹 정보 없음"))
                return

            raw_map = {}
            for e in rt_icons.directory.entries:
                e_id = e.id if e.name is None else str(e.name)
                data = self._extract_resource_data(pe, e)
                if data:
                    raw_map[e_id] = data

            groups = {}
            for grp in rt_groups.directory.entries:
                grp_id = grp.id if grp.name is None else str(grp.name)
                gdata = self._extract_resource_data(pe, grp)
                if not gdata or len(gdata) < 6:
                    continue

                _, _, id_count = struct.unpack("<HHH", gdata[:6])
                offset = 6
                entries = []
                for _ in range(id_count):
                    if offset + 14 > len(gdata):
                        break
                    w, h, col, _, planes, bpp, sz, icon_id = struct.unpack("<BBBBHHIH", gdata[offset:offset+14])
                    offset += 14

                    real_w = 256 if w == 0 else w
                    real_h = 256 if h == 0 else h

                    entries.append({
                        "width": real_w,
                        "height": real_h,
                        "raw_w": w, "raw_h": h, "colors": col,
                        "planes": planes, "bit_count": bpp,
                        "icon_id": icon_id
                    })
                if entries:
                    groups[grp_id] = entries

            self.current_pe = pe
            self.root.after(0, lambda: self._on_parse_complete(groups, raw_map, None))

        except Exception as e:
            self.root.after(0, lambda: self._on_parse_complete({}, {}, f"오류: {str(e)}"))

    def _on_parse_complete(self, groups, raw_map, err_msg):
        self.progress_bar.stop()
        self.progress_bar.pack_forget()

        if err_msg:
            self.lbl_count.config(text=err_msg)
            messagebox.showwarning("안내", err_msg)
            return

        self.group_cache = groups
        self.raw_icon_cache = raw_map
        self.lbl_count.config(text=f"총 {len(groups)}개 발견")
        self._set_btn_state(self.btn_select_all, True, "#1E293B", "#E2E8F0")

        w = self.canvas.winfo_width()
        avail_width = max(100, w - 20)
        cols = max(3, avail_width // 96) if w > 100 else 5
        self._last_cols = cols

        for grp_id, entries in self.group_cache.items():
            thumb = self._generate_thumbnail(entries, size=(64, 64))

            card = tk.Frame(self.grid_frame, bg="#FFFFFF", width=84, height=84, cursor="hand2")
            card.pack_propagate(False)

            lbl_img = tk.Label(card, image=thumb, bg="#FFFFFF")
            lbl_img.image = thumb
            lbl_img.place(relx=0.5, rely=0.5, anchor=tk.CENTER)

            for widget in (card, lbl_img):
                widget.bind("<Button-1>", lambda e, gid=grp_id: self._on_card_click(e, gid))

            self.tile_widgets[grp_id] = {"card": card, "lbl_img": lbl_img}
            self.tk_cache.append(thumb)

        self._relayout_grid(cols)
        self.canvas.yview_moveto(0.0)

        if self.group_cache:
            first_id = list(self.group_cache.keys())[0]
            self.selected_grp_ids = {first_id}
            self._inspect_icon(first_id)
            self._refresh_selection_visuals()

    def _generate_thumbnail(self, entries, size=(64, 64)):
        sorted_entries = sorted(entries, key=lambda x: x["width"], reverse=True)
        img = None
        for item in sorted_entries:
            raw = self.raw_icon_cache.get(item["icon_id"])
            if not raw:
                continue
            img = decode_icon_layer_to_image(raw, item)
            if img and (img.width > 0 and img.height > 0):
                break

        if not img:
            img = Image.new("RGBA", size, (0, 0, 0, 0))
        else:
            img.thumbnail(size, Image.Resampling.LANCZOS)

        return ImageTk.PhotoImage(img)

    def _on_card_click(self, event, grp_id):
        if event.state & 0x0004:
            if grp_id in self.selected_grp_ids:
                self.selected_grp_ids.remove(grp_id)
            else:
                self.selected_grp_ids.add(grp_id)
        else:
            self.selected_grp_ids = {grp_id}

        self._inspect_icon(grp_id)
        self._refresh_selection_visuals()

    def _refresh_selection_visuals(self):
        for gid, widgets in self.tile_widgets.items():
            card = widgets["card"]
            is_sel = gid in self.selected_grp_ids
            bg_col = "#E0E7FF" if is_sel else "#FFFFFF"

            card.config(bg=bg_col, highlightbackground="#4F46E5" if is_sel else "#E2E8F0", highlightthickness=2 if is_sel else 1)
            widgets["lbl_img"].config(bg=bg_col)

        count = len(self.selected_grp_ids)
        self.lbl_selected_status.config(text=f"{count}개 선택됨")
        
        has_sel = count > 0
        self._set_btn_state(self.btn_batch_ico, has_sel, "#4F46E5", "white", is_bold=True)
        self._set_btn_state(self.btn_batch_png, has_sel, "#F1F5F9", "#334155")

    def _inspect_icon(self, grp_id):
        self.active_preview_id = grp_id
        entries = self.group_cache[grp_id]

        large_img = self._generate_thumbnail(entries, size=(80, 80))
        self.lbl_large_preview.config(image=large_img)
        self.lbl_large_preview.image = large_img

        has_uhd = any(e["width"] >= 256 for e in entries)
        uhd_tag = " [UHD 256px]" if has_uhd else " [표준]"
        self.lbl_inspect_title.config(text=f"아이콘 #{grp_id}  ({len(entries)}개 레이어){uhd_tag}")

        for row in self.layer_tree.get_children():
            self.layer_tree.delete(row)

        for e in sorted(entries, key=lambda x: x["width"], reverse=True):
            self.layer_tree.insert("", tk.END, values=(f"{e['width']} × {e['height']} px", f"{e['bit_count']} bit"))

        self._set_btn_state(self.btn_extract_one_ico, True, "#EEF2FF", "#4F46E5", is_bold=True)
        self._set_btn_state(self.btn_extract_one_png, True, "#F8FAFC", "#475569")

    def toggle_select_all(self):
        if len(self.selected_grp_ids) == len(self.group_cache):
            self.selected_grp_ids.clear()
            self.btn_select_all.config(text="전체 선택")
        else:
            self.selected_grp_ids = set(self.group_cache.keys())
            self.btn_select_all.config(text="선택 해제")
        self._refresh_selection_visuals()

    def extract_single(self, fmt):
        if not self.active_preview_id:
            return
        ext = ".ico" if fmt == "ico" else ".png"
        out_path = filedialog.asksaveasfilename(
            defaultextension=ext,
            filetypes=[(f"{fmt.upper()} 파일", f"*{ext}")],
            initialfile=f"icon_{self.active_preview_id}{ext}"
        )
        if not out_path:
            return

        entries = self.group_cache[self.active_preview_id]
        try:
            if fmt == "ico":
                self._write_smart_uhd_ico(entries, out_path)
            else:
                self._write_uhd_png(entries, out_path)
            messagebox.showinfo("완료", f"저장되었습니다:\n{out_path}")
        except Exception as e:
            messagebox.showerror("오류", f"저장 실패: {str(e)}")

    def batch_extract(self, fmt):
        if not self.selected_grp_ids:
            return
        target_dir = filedialog.askdirectory(title="저장할 폴더 선택")
        if not target_dir:
            return

        success_cnt = 0
        for gid in self.selected_grp_ids:
            entries = self.group_cache[gid]
            out_path = os.path.join(target_dir, f"icon_{gid}.{fmt}")
            try:
                if fmt == "ico":
                    self._write_smart_uhd_ico(entries, out_path)
                else:
                    self._write_uhd_png(entries, out_path)
                success_cnt += 1
            except Exception:
                continue

        messagebox.showinfo("완료", f"{success_cnt}개의 고화질 파일이 저장되었습니다.")

    def _write_smart_uhd_ico(self, entries, out_path):
        has_256 = any(e["width"] >= 256 for e in entries)

        if has_256:
            count = len(entries)
            ico_header = struct.pack("<HHH", 0, 1, count)
            offset = 6 + (16 * count)
            dir_bytes = bytearray()
            data_bytes = bytearray()

            for item in entries:
                raw = self.raw_icon_cache[item["icon_id"]]
                dir_bytes.extend(struct.pack(
                    "<BBBBHHII",
                    item["raw_w"], item["raw_h"], item["colors"], 0,
                    item["planes"], item["bit_count"], len(raw), offset
                ))
                data_bytes.extend(raw)
                offset += len(raw)

            with open(out_path, "wb") as f:
                f.write(ico_header)
                f.write(dir_bytes)
                f.write(data_bytes)
            return

        sorted_entries = sorted(entries, key=lambda x: x["width"], reverse=True)
        best_entry = sorted_entries[0]
        raw = self.raw_icon_cache[best_entry["icon_id"]]
        master_img = decode_icon_layer_to_image(raw, best_entry)
        if not master_img:
            raise ValueError("아이콘 레이어를 디코딩할 수 없습니다.")

        target_sizes = [256, 128, 64, 48, 32, 16]
        ico_imgs = []
        for s in target_sizes:
            if s == master_img.width and s == master_img.height:
                ico_imgs.append(master_img)
            else:
                resized = master_img.resize((s, s), Image.Resampling.LANCZOS)
                ico_imgs.append(resized)

        # 바이너리 직접 직렬화로 완전한 ICO 팩 구성
        images_data = []
        for img in ico_imgs:
            buf = io.BytesIO()
            img.save(buf, format="PNG")
            images_data.append((img.width, buf.getvalue()))

        count = len(images_data)
        header = struct.pack('<HHH', 0, 1, count)
        offset = 6 + (16 * count)
        dirs = bytearray()
        body = bytearray()

        for sz, bdata in images_data:
            wb = 0 if sz >= 256 else sz
            dirs.extend(struct.pack('<BBBBHHII', wb, wb, 0, 0, 1, 32, len(bdata), offset))
            body.extend(bdata)
            offset += len(bdata)

        with open(out_path, "wb") as f:
            f.write(header)
            f.write(dirs)
            f.write(body)

    def _write_uhd_png(self, entries, out_path):
        sorted_entries = sorted(entries, key=lambda x: x["width"], reverse=True)
        best = sorted_entries[0]
        raw = self.raw_icon_cache[best["icon_id"]]

        if raw.startswith(b'\x89PNG\r\n\x1a\n') and best["width"] >= 256:
            with open(out_path, "wb") as f:
                f.write(raw)
            return

        img = decode_icon_layer_to_image(raw, best)
        if not img:
            raise ValueError("아이콘 레이어를 PNG로 디코딩할 수 없습니다.")

        if img.width < 256:
            img = img.resize((256, 256), Image.Resampling.LANCZOS)

        img.save(out_path, "PNG")

if __name__ == "__main__":
    try:
        myappid = f'mycompany.iconstudio.pro.{APP_VERSION}'
        ctypes.windll.shell32.SetCurrentProcessExplicitAppUserModelID(myappid)
    except Exception:
        pass

    if HAS_DND:
        root = TkinterDnD.Tk()
    else:
        root = tk.Tk()
    app = ModernIconStudio(root)
    root.mainloop()