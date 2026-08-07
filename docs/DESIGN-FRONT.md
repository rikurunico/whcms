# WHMCS Client Area Clone — "Twenty-One" theme spec

Reference: a stock WHMCS installation running the default client theme
Underlying platform: **WHMCS** (WHMCompleteSolution)
Active theme/template: **"twenty-one"** (WHMCS "Twenty-One" — the modern default client theme)
Framework: **Bootstrap 4** + Font Awesome 5 (Pro icon set)

This document specifies every visual and structural detail needed to reproduce the
client area with ~100% fidelity.

---

## 1. Tech Stack & Global Assets

### CSS / JS bundles (load order matters)
```
/assets/fonts/css/open-sans-family.css        # Open Sans webfont
/templates/twenty-one/css/all.min.css         # Bootstrap 4 + base theme
/templates/twenty-one/css/theme.min.css       # Twenty-One theme styles
/assets/fonts/css/fontawesome.min.css
/assets/fonts/css/fontawesome-solid.min.css   # fas
/assets/fonts/css/fontawesome-regular.min.css # far
/assets/fonts/css/fontawesome-light.min.css   # fal
/assets/fonts/css/fontawesome-brands.min.css  # fab
/assets/fonts/css/fontawesome-duotone.min.css # fad
/templates/twenty-one/css/custom.css          # per-install overrides

# JS
/templates/twenty-one/js/scripts.min.js       # theme JS (Bootstrap bundle, cart, etc.)
```

### Meta
```html
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
```

---

## 2. Design Tokens

### Typography
| Token          | Value                                  |
|----------------|----------------------------------------|
| Base font      | `"Open Sans", sans-serif`              |
| Base font-size | `14px`                                 |
| Base line-height | `21px` (1.5)                         |
| Body text color| `#212529` (rgb 33,37,41)               |
| Link color     | `#444444` (rgb 68,68,68)               |
| Nav link size  | `12.6px` (0.9rem), weight `400`        |
| `h1` (page title) | ~`36px`, weight `300` (light)       |
| `h2`           | `28px`, weight `500`, color `#212529`  |
| `h3` (card title) | ~`20px`                             |

### Colors
| Role                | Hex        | RGB               |
|---------------------|------------|-------------------|
| Body background     | `#F1F1F1`  | 241,241,241       |
| Header background   | `#FFFFFF`  | 255,255,255       |
| **Primary** (buttons/links accents) | `#336699` | 51,102,153 |
| **Success** (order/checkout) | `#218739` | 33,135,57 |
| Info badge (cart)   | Bootstrap `badge-info` (teal `#17a2b8`) |
| Breadcrumb bar bg   | `#E9ECEF`  | 233,236,239       |
| Breadcrumb text     | `#212529`  | 33,37,41          |
| Footer background   | `#404040`  | 64,64,64          |
| Footer text         | `#EEEEEE`  | 238,238,238       |
| Default btn border/text | `#A5A5A5` | 165,165,165    |
| Order Summary header bg | dark gray `#404040`-ish |          |

### Buttons
```
.btn-primary  → bg #336699, border #336699, color #fff, radius 3.5px,
                padding 5.25px 10.5px, font 14px/400
.btn-success  → bg #218739, border #218739, color #fff  (used for Order Now / Checkout)
.btn-default  → bg #fff, border #A5A5A5, color #A5A5A5   (search icon, secondary)
.btn-outline-primary → transparent bg, #336699 border+text (e.g. "Browse Products")
.btn-block    → full width (used on product cards)
Border radius default: 3.5px
```

### Layout
| Token             | Value    |
|-------------------|----------|
| Container max-width | `1140px` (Bootstrap 4 `.container`) |
| Grid              | Bootstrap 4 12-col |
| Card margin       | `.mb-3` |
| Card body padding | `.p-lg-4 .p-xl-5` on product cards |

---

## 3. Page Skeleton (every page)

```
<header id="header" class="header">          → white bar: logo + KB search + cart
  <nav navbar navbar-light>                  → main horizontal menu (dark links)
<breadcrumb bar>                             → light gray strip, e.g. "Portal Home"
<main .container>                            → page content
<footer id="footer" class="footer">          → dark gray: language switch, links, copyright
```

---

## 4. Header

Structure (exact):
```html
<header id="header" class="header">
  <div class="navbar navbar-light">
    <div class="container">
      <!-- Logo -->
      <a class="navbar-brand mr-3" href="/index.php">
        <img src="/assets/img/logo.png"
             alt="WHMCS Logo" class="logo-img">
      </a>

      <!-- Knowledgebase search (hidden below xl) -->
      <form method="post" action="/index.php?rp=/knowledgebase/search"
            class="form-inline ml-auto">
        <input type="hidden" name="token" value="...">
        <div class="input-group search d-none d-xl-flex">
          <div class="input-group-prepend">
            <button class="btn btn-default" type="submit">
              <i class="fas fa-search"></i>
            </button>
          </div>
          <input class="form-control appended-form-control font-weight-light"
                 type="text" name="search"
                 placeholder="Search our knowledgebase...">
        </div>
      </form>

      <!-- Toolbar: cart -->
      <ul class="navbar-nav toolbar">
        <li class="nav-item ml-3">
          <a class="btn nav-link cart-btn" href="/cart.php?a=view">
            <i class="far fa-shopping-cart fa-fw"></i>
            <span id="cartItemCount" class="badge badge-info">0</span>
            <span class="sr-only">Shopping Cart</span>
          </a>
        </li>
      </ul>
    </div>
  </div>
</header>
```
- Logo is the **WHMCS** cloud wordmark; replace `/assets/img/logo.png`.
- Search bar sits between logo and cart, right-aligned (`ml-auto`), visually a
  rounded pill with the magnifier button on the **left** (input-group-prepend).
- Cart icon = `far fa-shopping-cart` with a teal `badge-info` count bubble.

---

## 5. Primary Navigation Menu

Horizontal menu directly under the header (dark text on white). Items and links:

| Label            | Href                                        | Dropdown |
|------------------|---------------------------------------------|----------|
| Home             | `/index.php`                                | no       |
| Store            | `#`                                         | **yes**  |
| Announcements    | `/index.php?rp=/announcements`              | no       |
| Knowledgebase    | `/index.php?rp=/knowledgebase`              | no       |
| Network Status   | `/serverstatus.php`                         | no       |
| Contact Us       | `/contact.php`                              | no       |
| **Account** (right-aligned) | `#`                              | **yes**  |
| More (overflow)  | `#`                                         | yes (collapses items on small screens) |

**Store dropdown:**
- Browse All → `/index.php?rp=/store`
- cPanel Hosting Singapore - Pro → `/index.php?rp=/store/cloud-hosting-singapore`
- cPanel Hosting Indonesia - Pro → `/index.php?rp=/store/cpanel-hosting-indonesia-pro`
- Register a New Domain → `/cart.php?a=add&domain=register`
- Transfer Domains to Us → `/cart.php?a=add&domain=transfer`

**Account dropdown (right side):**
- Login
- Register
- Forgot Password?

(When logged in this becomes: Dashboard, My Services, My Domains, My Invoices,
My Emails, Edit Account Details, Logout — standard WHMCS.)

Nav link styling: font-size `0.9rem`, weight 400, dark gray; active/hover uses
the primary color underline/highlight from the theme.

---

## 6. Breadcrumb Bar

A full-width light gray strip (`#E9ECEF`) below the nav. Left-aligned text inside
`.container`. Example content: `Portal Home` (home), `Shopping Cart` (store/cart).
Text color `#212529`, small size.

---

## 7. Home Page (`/index.php`)

### 7a. Domain search hero
- Centered `<h2>`: **"Secure your domain name"** (28px, weight 500).
- White card containing a large multi-line `<textarea>`-style search box with
  placeholder:
  > "Search by keyword, description, or domain.
  > For example: "Fun business events ", "A comprehensive financial planning service for professionals", or "example.com"."
- Bottom-right of the box: two buttons side by side
  - **Search** (`.btn-primary`, blue, with `fas fa-magic`/sparkle icon)
  - **Transfer** (`.btn-success`, green)
- Below the box (left): three controls in a row:
  - `Include TLDs` dropdown
  - `Maximum Length` dropdown
  - `Safe Search` checkbox (checked by default)
- Right-aligned link: **"View all pricing"** (primary color).

### 7b. "Browse our Products/Services"
- Centered `<h2>` heading.
- 3-column responsive grid (`col-lg-4`) of white cards (`.card.mb-3`). Each card:
```html
<div class="card mb-3">
  <div class="card-body p-lg-4 p-xl-5 text-center">
    <h3 class="card-title pricing-card-title">cPanel Hosting Singapore - Pro</h3>
    <p></p>
    <a href="/index.php?rp=/store/cloud-hosting-singapore"
       class="btn btn-block btn-outline-primary">Browse Products</a>
  </div>
</div>
```
Cards shown (in order):
  1. **cPanel Hosting Singapore - Pro** → Browse Products
  2. **Register a New Domain** — subtitle "Secure your domain name by registering it today" → **Domain Search** button
  3. **Transfer Your Domain** — subtitle "Transfer now to extend your domain by 1 year" → **Transfer Your Domain** button
  4. **cPanel Hosting Indonesia - Pro** → Browse Products
  (Product-category cards + the fixed Register/Transfer cards.)

### 7c. "How can we help today"
- Centered `<h2>`.
- Row of 5 icon tiles (white cards, centered), each with a **colored top border**
  and a large gray Font Awesome icon above a label:
  | Tile | Icon | Top border color |
  |------|------|------------------|
  | Announcements | `fa-bullhorn` | teal/blue |
  | Network Status | `fa-server` | red |
  | Knowledgebase | `fa-book`/`fa-book-open` | yellow |
  | Downloads | `fa-download`/`fa-inbox` | gray |
  | Submit a Ticket | `fa-life-ring` | green |

### 7d. "Your Account"
- Centered `<h2>`.
- Row of 5 icon tiles (dark top border), same tile style:
  | Tile | Icon |
  |------|------|
  | Your Account | `fa-home` |
  | Manage Services | `fa-cubes` |
  | Manage Domains | `fa-globe` |
  | Support Requests | `fa-comments` |
  | Make a Payment | `fa-credit-card` |

---

## 8. Store / Product Group Page (`/index.php?rp=/store/<slug>`)

Two-column layout: **left sidebar (col-md-3) + main content (col-md-9)**.

### Left sidebar
Two collapsible cards, each with a `.card-header` (icon + title + chevron) and a
`.list-group.collapsable-card-body`:

**Categories** (`fa-shopping-cart` icon):
- cPanel Hosting Singapore - Pro (active item = solid **primary #336699** bg, white text)
- cPanel Hosting Indonesia - Pro

List items:
```html
<a href="/index.php?rp=/store/cloud-hosting-singapore"
   class="list-group-item list-group-item-action"
   id="Secondary_Sidebar-Categories-...">cPanel Hosting Singapore - Pro</a>
```

**Actions** (`fa-plus` icon):
- Register a New Domain (`fa-globe`)
- Transfer in a Domain (`fa-arrow-right`/exchange)
- View Cart (`fa-shopping-cart`)

### Main content
- Page `<h1>` (light weight): **"cPanel Hosting Singapore - Pro"**
- Product cards in a **2-column grid** (`col-md-6`). Each product card:
  - Card header with product name (e.g. `50GB-SG`) — light gray header.
  - Card body split: **left = feature list**, **right = price + Order Now**.
  - Feature list: each line has a **bold value** + normal label, e.g.
    `**50GB NVME SSD** Storage`, `**Unlimited** Bandwidth`, `**Unlimited** Addon Domain`,
    `Domain Parking`, `Sub Domain`, `Database MySQL`, `Akun FTP`, `Akun Email`,
    `**2x per Hari** Backup`, `**4GB** RAM`, `**2 Core** CPU`, `**250MB/s** IO Speed`,
    `Cloud Server Deploy`.
  - Price block (right, centered): large **`Rp50,000 IDR`** + smaller **"Monthly"** below.
  - **Order Now** button = `.btn-success` (green) with `fa-shopping-cart` icon.

Example products on the Singapore group: `50GB-SG` (Rp50,000/mo), `20GB-SG`
(Rp100,000/mo), `UNLIMITED-SG` (Rp100,000/mo).

---

## 9. Cart / Checkout Page (`/cart.php?a=view`)

Three-column layout: **sidebar (col-md-3) + cart (col-md-6) + order summary (col-md-3)**.

- Same left sidebar as the store page (Categories + Actions). "View Cart" action
  shows as the active (primary bg) item.
- Center: `<h1>` **"Review & Checkout"**.
  - Cart table header bar: **primary blue `#336699`** background, white text,
    columns `Product/Options` (left) and `Price/Cycle` (right).
  - Empty state message centered: **"Your Shopping Cart is Empty"**.
  - Below: tabbed panel **"Apply Promo Code"** → text input placeholder
    "Enter promo code if you have one" + full-width **"Validate Code"** button
    (default/outline).
- Right: **Order Summary** card.
  - Card header dark gray (`#404040`) with centered white **"Order Summary"**.
  - Rows: `Subtotal … Rp0 IDR`, `Totals`.
  - Large total: **`Rp0 IDR`** + small **"Total Due Today"**.
  - Green **"Checkout →"** button (`.btn-success .btn-block .btn-lg`).
  - Below: **"Continue Shopping"** text link (centered, small).

---

## 10. Login Page (`/index.php?rp=/login`)

- Centered white card (`col-md-6` centered), `.card` with padding.
- `<h1>` **"Login"** (light weight) + muted subtitle **"Sign in to your account to continue."**
- Fields (Bootstrap input-group with left icon):
  - **Email Address** — icon `fa-user`, placeholder `name@example.com`.
  - **Password** — icon `fa-key`, placeholder `Password`, right-side eye toggle
    (`fa-eye`) to show/hide. Right-aligned **"Forgot Password?"** link above field.
  - **CAPTCHA**: instructional text
    "Please enter the characters you see in the image below into the text box provided.
    This is required to prevent automated submissions." + distorted-text image + input box.
    *(Do not attempt to auto-solve; render the image + input only.)*
- **Login** button (`.btn-primary`, blue) bottom-left; **"Remember Me"** checkbox bottom-right.
- Card footer: **"Not registered? Create account"** (Create account = primary link).

---

## 11. Footer

```html
<footer id="footer" class="footer">    <!-- bg #404040, text #EEEEEE -->
  <div class="container">
    <!-- Language/currency switcher (right on lg, centered on mobile) -->
    <ul class="list-inline mb-7 text-center float-lg-right">
      <li class="list-inline-item">
        <button type="button" class="btn" data-toggle="modal"
                data-target="#modalChooseLanguage">
          <div class="d-inline-block align-middle"><div class="iti-flag us"></div></div>
          English / Rp IDR
        </button>
      </li>
    </ul>
    <!-- Links -->
    <ul class="nav justify-content-center justify-content-lg-start mb-7">
      <li class="nav-item"><a class="nav-link" href="/contact.php">Contact Us</a></li>
    </ul>
    <p class="copyright mb-0">
      Copyright © 2026 WHMCS. All Rights Reserved.
    </p>
  </div>
</footer>
```
- Uses a US flag sprite (`iti-flag us`) + label **"English / Rp IDR"**, opens a
  language-chooser Bootstrap modal (`#modalChooseLanguage`).
- Copyright year is dynamic (currently **2026**).

### "Powered by" strip
Above the footer, on most content pages there is a centered line:
**"Powered by WHMCompleteSolution"** (WHMCompleteSolution = primary-color link).
Keep or rebrand as needed.

---

## 12. Icons Reference (Font Awesome 5)

| Usage | Class |
|-------|-------|
| Header search | `fas fa-search` |
| Cart | `far fa-shopping-cart fa-fw` |
| Sidebar Categories | `fas fa-shopping-cart` |
| Sidebar Actions | `fas fa-plus` |
| Register domain | `fas fa-globe` |
| Transfer domain | `fas fa-arrow-right` (exchange) |
| Order Now / Checkout | `fas fa-shopping-cart` |
| Login user | `fas fa-user` |
| Password | `fas fa-key` |
| Password reveal | `far fa-eye` |
| Help tiles | `fa-bullhorn`, `fa-server`, `fa-book`, `fa-download`, `fa-life-ring` |
| Account tiles | `fa-home`, `fa-cubes`, `fa-globe`, `fa-comments`, `fa-credit-card` |
| Collapse chevron | `fa-chevron-up` / `fa-chevron-down` |

---

## 13. Currency & Locale

- Currency: **IDR**, displayed as `Rp<amount> IDR` (e.g. `Rp50,000 IDR`), thousands
  separated by commas, no decimals shown.
- Billing cycle label under price: **"Monthly"**.
- Default locale: English; currency switch available in footer modal.

---

## 14. Responsive Behavior

- Bootstrap 4 breakpoints. Container caps at `1140px`.
- KB search bar hidden below `xl` (`d-none d-xl-flex`).
- Nav collapses into hamburger + "More" overflow on small screens.
- Product/summary grids stack on `sm`/`xs`.
- Footer language block: right-floated on `lg+`, centered on mobile.

---

## 15. Implementation Checklist

- [ ] Load Open Sans + Font Awesome 5 (solid/regular/light/brands).
- [ ] Set design tokens (colors, radii, spacing) as CSS variables/SCSS.
- [ ] Build shared header (logo, KB search, cart badge), nav, breadcrumb, footer partials.
- [ ] Home: domain hero, product/service cards, two 5-tile help grids.
- [ ] Store: sidebar (Categories + Actions) + 2-col product cards with feature lists.
- [ ] Cart: 3-col layout, blue table header, promo tab, dark Order Summary panel.
- [ ] Login/Register/Forgot pages with input-group icons + captcha placeholder.
- [ ] IDR currency formatting `Rp#.###,-` + "Monthly" cycle.
- [ ] Language/currency footer modal.
- [ ] Responsive rules per Bootstrap 4 breakpoints.