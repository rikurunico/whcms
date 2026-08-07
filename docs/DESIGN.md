# Panduan Clone WHMCS Admin Area ("Blend" Theme)

> Tujuan: spesifikasi visual & struktural untuk mengkloning tampilan WHMCS Admin Area
> (tema **blend**) hingga mirip ~100%.
> Basis: WHMCS v9.0.2, tema admin `blend`.

---

## 1. Teknologi & Aset

| Item | Nilai |
|---|---|
| Framework CSS | Bootstrap 3 (`panel`, `btn`, `navbar`, `label`, grid `col-*`, `nav-tabs`) |
| Tema admin | `templates/blend/` |
| Font utama | **"Open Sans", sans-serif** |
| Ikon | **Font Awesome 5** (solid/regular/light/brands/duotone) |
| Font body | `14px`, teks `#333333`, bg `#F6F6F6` |
| CSS | `blend/css/all.min.css`, `blend/css/theme.min.css` |
| JS | `blend/js/vendor.min.js` (jQuery + plugin), `blend/js/scripts.min.js` |
| Logo | `templates/blend/images/logo.png` — render **125×28 px** (teks "WHMCS" putih, gear hijau) |
| Mata uang | **IDR** — prefix `Rp`, suffix `IDR`, format ribuan `1,234` → `Rp185,000 IDR` |
| Format tanggal | `DD/MM/YYYY` & `DD/MM/YYYY HH:MM` |

---

## 2. Palet Warna (Design Tokens)

| Token | Hex | Penggunaan |
|---|---|---|
| Navy Primary | `#1A4D80` | Top navbar, header tabel, bar total, footer |
| Sidebar link | `#202F60` | Link sidebar (underline) & tab non-aktif |
| Sidebar header | `#444444` | Judul grup sidebar (bold) |
| Btn Primary | bg `#337AB7`, border `#2E6DA4` | Tombol utama (radius 4–6px) |
| Help text | `#337AB7` (biru), ~12px | Teks bantuan di bawah/samping field |
| Teks body | `#333333` | Teks umum |
| Background | `#F6F6F6` | Body |
| Panel bg | `#FFFFFF` | Card/panel |
| Border input | `#CCCCCC` | Border field form |
| Border baris tabel | `#EBEBEB` | Garis antar baris |

### Badge status (label): `radius 3px; font-size 10px; weight 400; padding 1px 3px 2px; color #fff`
| Status | Background |
|---|---|
| Active | `#46A546` hijau |
| Pending | `#F89406` oranye |
| Suspended | `#0768B8` biru |
| Terminated | `#C43C35` merah |
| Cancelled | `#BFBFBF` abu |
| Fraud | `#000000` hitam |
| "New" (lembut) | bg `#E2EFDA`, teks `#547E4E` |

### Teks status inline (tabel): Complete/Paid/Active=hijau · Incomplete/Unpaid/Overdue=merah · Pending=oranye · Cancelled=abu.

---

## 3. Layout Global

```
┌──────────────────────────────────────────────────┐
│ TOP NAVBAR (fixed, navy #1A4D80, 45px)            │
├──────────┬───────────────────────────────────────┤
│ SIDEBAR  │ CONTENT: <h1> judul (19.6px/400)       │
│ (±195px) │  → tabs/filter → tabel/form/widget     │
├──────────┴───────────────────────────────────────┤
│ FOOTER (navy, teks putih)                          │
│ DEV LICENSE BANNER (bg kuning — hanya lisensi dev) │
└──────────────────────────────────────────────────┘
```
> Pengecualian: **System Settings** (SmartSearch) tanpa sidebar klasik — lihat §7.

### 3.1 Top Navbar
- Navy `#1A4D80`, 45px, fixed. Kiri→kanan: logo → tombol `+` (Add New) → menu utama.
- **Menu utama**: Clients · Orders · Billing · Support · Reports · Utilities · Addons.
- **Ikon kanan**: 🔍 Search · ⚙️ Configuration · ⬇️ Updates(kuning bila ada update) · 🔧 Utilities/tools · 👤 Account · ❓ Help.
- Tombol `+`: New Client · New Order · New Invoice · New Quote · New Ticket.

### 3.2 Sidebar Kiri (halaman klasik)
- Lebar ±195px. Grup: header **bold #444** 14px (padding `3px 10px`) + ikon FA.
- Link: `#202F60`, 14px, `underline`, padding `3px 10px`. Kontekstual per modul.
- Umum berisi: (Modul-links) · Advanced Search (2 dropdown + input) · Staff Online · `« Minimise Sidebar`.

### 3.3 Footer
- Kiri: `Copyright © WHMCS {tahun}. All Rights Reserved.` Kanan: `Report a Bug | Documentation | Contact Us`. Bg navy, teks putih.

---

## 4. Komponen Reusable

- **Panel/Card**: `bg #fff; border-radius 4px; box-shadow 0 1px 1px rgba(0,0,0,.05)`. `.panel-heading` padding `10px 15px`, 14px/400.
- **Tabel `.datatable`**: header **navy #1A4D80, putih, bold(700), center, padding 4px**; sel padding 3px, border-bottom `#EBEBEB`; kolom sortable (▼); kolom-1 checkbox untuk bulk.
- **Tombol Primary**: bg `#337AB7`, border `#2E6DA4`, putih, radius 4–6px, padding `5–6px 12px`.
- **Tab (`.nav-tabs`)**: aktif = putih, `border-radius 4px 4px 0 0`, border `#ddd`, teks `#555`; non-aktif = bg `#EFEFEF`, teks `#202F60`.
- **Form input**: `border 1px solid #CCC; border-radius 2px; padding 4px 8px; height 30px; width 300px; font 14px`. Checkbox ±13×13px. Textarea sama.
- **Form settings = `<table class="form">`**: label **rata kanan** (`text-align:right`, weight 400, lebar ±280px) | field di tengah | **help text biru** (`#337AB7`, ~12px) di samping/bawah. Footer: `Save Changes`(biru) + `Cancel Changes` (centered).
- **Search/Filter panel**: tab collapsible di atas tabel list.
- **Status counter bar** (list Invoices): `Paid(hijau) Unpaid(merah) Overdue(bold)`.
- **Jump to Page**: kanan atas — "N Records Found, Showing X to Y" + toggle ON/OFF (mis. Hide Inactive) + dropdown halaman.
- **Bulk Action Bar**: "With Selected:" → Pin/Unpin/Merge/Close(abu) + Delete/Block(**merah**), di atas & bawah tabel.
- **Pagination**: `« Previous Page | [1] | Next Page »` (kotak aktif navy).
- **Alert box**: kuning muda + ikon ℹ️ bundar.
- **Password Gate**: card tengah "Confirm password to continue" + field + tombol biru.

---

## 5. Peta Navigasi Lengkap (Menu → Item → URL, relatif ke admin path WHMCS)

**Add New(+)**: clientsadd.php · ordersadd.php · index.php?rp=/billing/invoice/new · quotes.php?action=manage · supporttickets.php?action=open

**Clients**: clients.php · index.php?rp=/user/list · clientsadd.php · index.php?rp=/services (shared/reseller/server/other) · index.php?rp=/addons · index.php?rp=/domains · cancelrequests.php · affiliates.php

**Orders**: orders.php (+?status=Pending/Active/Fraud/Cancelled) · ordersadd.php

**Billing**: transactions.php · invoices.php (?status=Paid/Draft/Unpaid/Overdue/Cancelled/Refunded/Collections/Payment%20Pending) · billableitems.php · quotes.php · offlineccprocessing.php · index.php?rp=/billing/disputes · gatewaylog.php

**Support**: supportcenter.php · supporttickets.php (?view=flagged/active/Open/Answered/Customer-Reply/On Hold/In Progress/Closed) · supportticketpredefinedreplies.php · supportannouncements.php · supportdownloads.php · supportkb.php · networkissues.php

**Reports**: reports.php (+?report=daily_performance/income_forecast/annual_income_report/new_customers/ticket_feedback_scores/pdf_batch)

**Utilities**: update.php · whmcsconnect.php · automationstatus.php · modulequeue.php · index.php?rp=/utilities/sitejet/builder · index.php?rp=/utilities/tools/tldsync/import · index.php?rp=/utilities/tools/email/campaigns · utilitiesemailmarketer.php · utilitieslinktracking.php · calendar.php · todolist.php · whois.php · utilitiesresolvercheck.php · systemintegrationcode.php · System(systemdatabase.php/systemcleanup.php/systemphpinfo.php/php-compat)

**Configuration(⚙️)**: index.php?rp=/setup · index.php?rp=/apps · configadmins.php · systemhealthandupdates.php · index.php?rp=/getting-started · systemactivitylog.php

**Account(👤)**: myaccount.php · My Notes(modal) · ../ (Client Area) · logout.php

**Help(❓)**: docs.whmcs.com · systemsupportrequest.php · Community Forums · What's New · index.php?rp=/help/license

---

## 6. Katalog Halaman Manajemen (deskripsi layout)

### 6.1 Dashboard (`index.php`)
- 4 **stat card berwarna**: Pending Orders(hijau/keranjang) · Tickets Waiting(pink/chat) · Pending Cancellations(oranye/larangan) · Pending Module Actions(teal/peringatan).
- Widget grid (collapse/refresh/close `↻ ∧ ×`): System Overview (grafik garis + toggle Today/30d/1y), Automation Overview (6 sparkline), Billing (Today/Month/Year/All Time), Activity, Network Status, Staff Online, To-Do List (badge PENDING + due), Client Activity.
- Sidebar: Shortcuts, System Information, Advanced Search.

### 6.2 List Page generik (Clients/Orders/Products/Invoices)
Pola: `<h1>` → tab Search/Filter → "N Records Found" + Jump to Page → tabel `.datatable` (header navy, checkbox, badge status) → pagination.
- **Clients**: ID·First·Last·Company·Email·Services·Created·Status. Filter horizontal + ikon kaca hijau + `+Advanced` + Search(biru).
- **Orders**: ID·Order#·Date·Client·Payment Method·Total·Payment Status(warna)·Status(warna)·aksi.
- **Products/Services (list)**: ID·Product·Domain(+www)·Client·Price·Billing Cycle·Next Due·Status(badge)·`+` expand.
- **Invoices**: counter bar; Invoice#·Client·Invoice Date·Due Date·Last Capture·Total·Payment Method·Status·tombol View/Edit/Refund/Cancel.

### 6.3 Client Profile (`clientssummary.php?userid=`)
- Dropdown pemilih klien. **Tab**: Summary·Profile·Users·Contacts·Products/Services·Domains·Billable Items·Invoices·Quotes·Transactions·Tickets·Emails·Notes(N)·(▾).
- Baris flag: Exempt from Tax · Auto CC Processing · Send Overdue Reminders · Apply Late Fees (Yes/No berwarna).
- **4 kolom panel**: (1) Clients Information + Login as Owner + Contacts + Pay Methods; (2) Invoices/Billing + Other Information; (3) Products/Services + Files + Recent Emails; (4) Other Actions (aksi destruktif merah) + Send Email + Admin Notes.

### 6.4 Invoice Detail (`invoices.php?action=edit&id=`)
- Judul + tombol kanan: View Invoice/View as Client/Print▾/Download▾. **Tab**: Summary·Add Payment·Options·Credit·Refund·Notes.
- Kiri: tabel ringkas; kanan: **status besar berwarna** (PAID hijau) + metode + dropdown + Send Email + Attempt Capture(hijau)/Mark Cancelled/Mark Paid(biru).
- Alert info bila terkunci. Invoice Items editable. Total di **bar navy**. Ledger table.

### 6.5 Support Tickets (`supporttickets.php`)
- Tab Search/Filter + Auto Refresh. Bulk bar. Tabel: checkbox·Department·Subject·Requestor·Status·Last Reply.
- Sidebar: Filter Tickets (Status/Department/Subject/Email + Filter»), Tag Cloud, Network Issues, Advanced Search.

### 6.6 Reports (`reports.php`)
- **Grup tombol per kategori**: General(biru/teal) · Billing/Income/Clients/Support(putih outline) · Exports(abu gelap) · System. Sidebar daftar report alfabetis.

### 6.7 Transactions (`transactions.php`)
- Tab Search/Filter + Add Transaction. **Grafik area hijau** + 3 stat card (Total Income/Fees/Expenditure, ikon abu, `↑x%`). Tabel: Client·Date·Payment Method·Description·Amount In·Fees·Amount Out·aksi.

---

## 7. System Settings (SmartSearch UI — layout modern)
`index.php?rp=/setup`
- Full-width tanpa sidebar klasik. `<h1>` + subteks + link "view setup tasks" + **progress bar hijau** (%).
- Kiri: Search + menu kategori (All[aktif, highlight hijau muda]/System/Apps & Integrations/User Management/Products & Services/Support/API & Security) + Recently Visited.
- Kanan: "All Settings" + sort dropdown + **grid kartu 3 kolom**: ikon garis abu besar, judul(link biru) + badge (`UPDATED` biru / `NEW SERVICE`,`New` hijau) + deskripsi abu.

---

## 8. Halaman Configuration (klasik) — layout & sidebar

### 8.1 Sidebar Configuration (grup → item)
- **Configuration**: General Settings, Apps & Integrations, Sign-In Integrations, Automation Settings, MarketConnect, Notifications, Storage Settings, Application Links, OpenID Connect, Email Templates, Addon Modules, Client Groups, Custom Client Fields, Fraud Protection.
- **Staff Management**: Administrator Users, Administrator Roles, Two-Factor Authentication, Manage API Credentials.
- **Payments**: Currencies, Payment Gateways, Tax Configuration, Promotions.
- **Products/Services**: Products/Services, Configurable Options, Product Addons, Product Bundles, Domain Pricing, Domain Registrars, Servers.
- **Support**: Support Departments, Ticket Statuses, Escalation Rules, Spam Control.
- **Other**: Order Statuses, Security Questions, Banned IPs, Banned Emails, Database Backups.

### 8.2 General Settings (`configgeneral.php`)
- **Tab horizontal**: General · Localisation · Ordering · Domains · Mail · Support · Invoices · Credit · Affiliates · Security · Social · Other.
- Isi = `<table class="form">`: label rata-kanan · field · help text biru. Footer `Save Changes`/`Cancel Changes` (centered).
- Field khas tab General: Company Name, Email Address, Domain, Logo URL, Pay To Text(textarea), WHMCS System URL, System Theme(select), Limit Activity Log, Records per Page(select), Maintenance Mode(checkbox)+Message+Redirect, Friendly URLs (badge `SYSTEM-DETECTED` + refresh).

### 8.3 Administrators (`configadmins.php`)
- Deskripsi + tombol "Add New Administrator". Dua tabel: **Active** & **Inactive Administrators** — kolom Name·Email·Username·Admin Role·Assigned Departments·aksi(edit/hapus). *(Halaman ini di balik password gate.)*

### 8.4 Payment Gateways (`configgateways.php`)
- Info banner biru (+ "Visit Apps & Integrations") + banner promo. **Daftar gateway aktif = baris draggable**: handle ✛ · nama · (Module type) · pencil edit.

### 8.5 Email Templates (`configemailtemplates.php`)
- Deskripsi + "Create New Email Template" & "Manage Languages". **Dua kolom berisi tabel per kategori** (General/User/Invite/Admin Invite/Invoice Messages | Product/Service Messages ...). Kolom: Status(ikon centang hijau=aktif)·Template Name·aksi edit.

### 8.6 Products/Services config (`configproducts.php`)
- Deskripsi + tombol (Create New Group / Create New Product / Duplicate / Refresh Feature Status). **Carousel kartu promo** (dot pagination). Tabel dikelompokkan per **Product Group** (baris "Group Name:" draggable) — kolom Product Name·Type·Pay Type·Stock·Auto Setup·Features·aksi(drag/edit/hapus). Item bisa "(Hidden)".

### 8.7 Currencies (`configcurrencies.php`)
- Tabel currency: Currency Code·Prefix·Suffix·Format·Base Conv. Rate·aksi. Tombol Update Exchange Rates / Update Product Prices. Form "Add Additional Currency" (`table.form`).

---

## 9. Perilaku Interaktif
- Menu navbar & Configuration = dropdown. Widget dashboard collapse/refresh/close (posisi tersimpan per admin).
- Panel Search/Filter & Advanced Search collapsible. Tabel sortable + bulk-select. Toggle Jump to Page / Hide Inactive.
- Baris gateway & product group **drag-and-drop** untuk reorder. Password gate untuk halaman admin sensitif.

---

## 10. Checklist Clone (agar 100% mirip)
1. [ ] Open Sans + FA5; body 14px/#333, bg #f6f6f6.
2. [ ] Navbar navy #1A4D80 45px + logo 125×28 + 7 menu + 6 ikon kanan.
3. [ ] Sidebar 195px (header bold #444, link #202F60 underline).
4. [ ] Panel radius 4px + shadow halus.
5. [ ] `.datatable` header navy bold putih center; sel padding 3px, border #EBEBEB.
6. [ ] Badge status sesuai palet §2.
7. [ ] Tombol primary #337AB7. Form input 1px #CCC/radius 2px/h30px.
8. [ ] Form settings `table.form`: label rata-kanan + help text biru + Save/Cancel centered.
9. [ ] Footer navy + copyright + link kanan.
10. [ ] Dashboard 4 stat card + widget grid collapsible.
11. [ ] Client Profile: tab + 4 kolom panel.
12. [ ] Invoice detail: tab + status besar + items editable + ledger.
13. [ ] System Settings: grid kartu modern + progress bar + kategori kiri.
14. [ ] Reports: grup tombol berwarna. Transactions: grafik + 3 stat card.
15. [ ] Config: tab horizontal (General Settings), sidebar 6 grup, daftar gateway draggable, email templates multi-tabel, products grouped.
16. [ ] URL pola `admin_path/*.php` atau `index.php?rp=/...`.
