# quark

NeutronLinux'un paket yöneticisi. Native `.qpk` formatına ve native repo'ya
sahip, ama tek bir formata kilitli değil: Arch (`.pkg.tar.zst`) ve Debian
(`.deb`) paketlerini hem yerel dosyadan hem de doğrudan o distroların resmi
mirror'larından çekip kurabilir, ayrıca generic `.tar.gz` arşivlerini de
kurar.

```
quark :i <paket>                      önce native repoda arar, yoksa (aktifse) arch/debian'da
quark :i arch:<paket>                 doğrudan Arch repolarından
quark :i deb:<paket>                  doğrudan Debian repolarından
quark :i <dosya>.qpk|.pkg.tar.zst|.deb|.tar.gz   yerel dosyadan (format otomatik algılanır)
quark :r <paket>                      kaldır
quark :s <sorgu>                      tüm aktif kaynaklarda ara
quark :u                              tüm aktif kaynakların metadata'sını senkronize et
quark :up                             kurulu paketleri yükselt
quark :info <paket>                   paket detayı
quark :list                           kurulu paketleri listele

quark :export <profil>[.qprof] [dotfile ...]   sistemi bir .qprof'a klonla
quark :import <profil>.qprof                   bir .qprof'u bu sisteme uygula

quark :i --force ...                  glibc ABI kontrolünü atla (riski göze alarak)
quark :import --force ...             var olan dotfile'ların üzerine yaz
```

### Arch/Debian tercihi: hangisi daha az sorun çıkarır?

NeutronLinux'un kendi glibc'si sabit bir sürümde pinlenmiş (from-scratch bir
sistem olduğu için rolling-release değil). Bu yüzden:

- **Debian** genelde güvenli: stable dağıtımların glibc'si tipik olarak
  NeutronLinux'unkinden eski ya da yakın kalıyor.
- **Arch** rolling-release olduğu için sürekli en yeni glibc'yi kullanıyor -
  glibc'ye derin bağımlı paketler (ncurses, systemd-libs, gcc-libs gibi)
  sık sık NeutronLinux'un desteklediğinden daha yeni sembol istiyor ve
  ABI kontrolü tarafından reddediliyor (bkz. "Ortak davranış" altında
  glibc ABI uyumluluk kontrolü).

Kalıcı çözüm NeutronLinux'un kendi glibc'sini periyodik olarak güncel
tutmak (cross-toolchain'in glibc/gcc-pass2 aşamalarının yeniden
derlenmesini gerektirir - büyük bir iş). Kısa vadede reddedilen ama
güvenli olduğunu bildiğiniz bir paket için `--force` kullanılabilir.

## Kurulum

```sh
go build -o quark ./cmd/quark
```

Go 1.26+ ve internet erişimi (bağımlılıkları çekmek için) yeterli.
Cross-derleme için normal `GOOS`/`GOARCH` ortam değişkenleri geçerli - cgo
kullanılmıyor, tüm bağımlılıklar saf Go.

## Örnekler

```sh
# native repodan kurulum
quark :u
quark :i htop

# doğrudan Arch mirror'ından
quark :i arch:neofetch

# doğrudan Debian mirror'ından
quark :i deb:curl

# yerel dosyadan - format uzantıdan otomatik algılanır
quark :i ./benim-paketim-1.0.qpk
quark :i ./htop-3.3.0-1-x86_64.pkg.tar.zst
quark :i ./curl_7.88.1-10_amd64.deb
quark :i ./bagimsiz-arac-2.0.tar.gz

# arama, kaynağı etiketiyle gösterir
quark :s editor
#   [native] neutron-editor 1.2.0
#   [arch] neovim 0.10.1
#   [deb] vim 9.0.1378

quark :list
#   [native] neutron-editor 1.2.0 (qpk)
#   [arch] neovim 0.10.1 (pkg.tar.zst)
#   [foreign] bagimsiz-arac 2.0 (tar)

quark :r neofetch
```

## Config

`/etc/quark/quark.conf` (TOML). Hiç yoksa gömülü varsayılanlarla çalışır -
bkz. `quark.conf.example` için tam bir örnek. Her kaynak ayrı ayrı
`enabled`/mirror ile kontrol edilir:

```toml
root_dir  = "/"
cache_dir = "/var/cache/quark"

[sources.native]
enabled = true
mirror  = "https://repo.neutronlinux.org"

[sources.arch]
enabled = true
mirror  = "https://geo.mirror.pkgbuild.com"
repos   = ["core", "extra"]
arch    = "x86_64"

[sources.debian]
enabled    = true
mirror     = "http://deb.debian.org/debian"
suite      = "bookworm"
components = ["main"]
arch       = "amd64"
```

`--config <yol>` ile başka bir dosya belirtilebilir (testler ve chroot/imaj
inşası için kullanışlı - `root_dir`'i gerçek `/` yerine bir hazırlık
dizinine işaret edecek şekilde ayarlayın).

## Native `.qpk` formatı

Zstd sıkıştırmalı bir tar arşivi:

```
metadata.json          # {"name":..,"version":..,"description":..,"depends":[...]}
data/usr/bin/foo        # /usr/bin/foo'ya kurulacak dosya
data/etc/foo.conf        # /etc/foo.conf'a kurulacak dosya
```

`internal/pkgformat.Build(srcRoot, meta, destPath)` bir dizin ağacından
(`srcRoot/usr/bin/foo` şeklinde, hedef kök dizini birebir yansıtan) `.qpk`
üretir - repo tarafı için gereken tek araç budur, quark'ın kendisi sadece
okuma (`Install`) tarafını CLI'dan kullanır.

Native repo, `<mirror>/index.json` altında bir JSON dizisi + o dosyaları
barındıran düz bir HTTP(S) sunucusudur:

```json
[
  {
    "name": "htop",
    "version": "3.3.0",
    "description": "Interactive process viewer",
    "depends": ["ncurses"],
    "filename": "htop-3.3.0.qpk",
    "sha256": "..."
  }
]
```

## Sistem klonlama: `.qprof`

`quark :export`/`quark :import`, bir sistemi (kurulu paket listesi + seçili
dotfiles) tek bir `.qprof` dosyasına dondurup başka bir makinede (ya da
aynı makinenin taze bir kurulumunda) yeniden uygulamak için var.

```sh
# kurulu paketler + seçili dotfiles'ı dışa aktar (yollar $HOME'a göreli)
quark :export laptop-profili .zshrc .config/nvim .gitconfig
#   -> laptop-profili.qprof

# başka bir NeutronLinux kurulumunda içe aktar
quark :import laptop-profili.qprof
#   ✓ 12 paket kuruldu, 3 paket zaten güncel
#   ✓ 3 dotfile geri yüklendi, 0 dotfile atlandı
#   (var olan bir dotfile'ın üzerine yazmak için --force gerekir)
```

`.qprof`, `.qpk`'nın `metadata.json` + `data/` düzenini taklit eden gzip'li
bir tar arşivi:

```
manifest.json           # {"packages":[{"name","version","source","format"}...], "dotfiles":[...]}
dotfiles/.zshrc          # $HOME/.zshrc'nin kopyası
dotfiles/.config/nvim/   # $HOME/.config/nvim'in kopyası
```

`manifest.json`'daki her paket `source` alanıyla (native/arch/debian)
etiketlenir - `:import` bunu `quark :i`'nin zaten bildiği
`arch:`/`deb:`/native etiketleme düzenine geri çevirip aynı kurulum
mantığını (bağımlılık çözümü, glibc ABI kontrolü dahil) kullanır. `foreign`
kaynaklı bir paket (yerel bir dosyadan kurulmuş, hiçbir repo'yu izlemeyen)
yeniden kurulamaz - `:export` yine de onu manifest'e yazar (dışa aktarılan
profilin makinede gerçekte ne olduğunu eksiksiz yansıtması için), `:import`
ise bunu sessizce atlamak yerine açık bir hata olarak bildirir.

Bir paket ya da dotfile başarısız olursa tüm işlem durmaz - quark'ın hiçbir
yerinde paketler-arası transactional rollback yok (bkz. `core.Installer.
Commit`), ve tek bir bozuk girdi yüzünden zaten geri yüklenmiş her şeyi
atmak daha kötü olurdu. Bunun yerine her hata özete toplanır, işlem sonunda
tam bir "kaç paket kuruldu / atlandı / hata verdi" dökümü basılır ve
kısmi başarısızlık durumunda süreç sıfırdan farklı bir çıkış koduyla biter.

## Mimari

```
cmd/quark/                  CLI: argüman ayrıştırma, komut dispatch
internal/core/               Package/DB/Installer - format ve kaynaktan bağımsız çekirdek model
internal/config/              /etc/quark/quark.conf yükleme + varsayılanlar
internal/manager/             kaynakları+adaptörleri+DB'yi birbirine bağlayan üst seviye mantık
internal/pkgformat/           native .qpk okuma/yazma
internal/clone/                .qprof okuma/yazma (manifest + dotfiles) - :export/:import'ın ince katmanı
internal/adapters/
  arch/                       .pkg.tar.zst kurulumu (zstd tar + .PKGINFO)
  debian/                     .deb kurulumu (ar + control.tar.* + data.tar.*)
  generic/                    .tar.gz/.tar.xz/.tar.zst kurulumu (FHS algılama + --prefix)
internal/sources/
  native/                     index.json tabanlı native repo istemcisi
  arch/                       pacman sync db (.db.tar.gz) indirme + ayrıştırma
  debian/                     Packages.{gz,xz} indirme + ayrıştırma
internal/fetch/                ortak HTTP indirme + ilerleme çubuğu + sha256 doğrulama + cache
internal/ui/                   renkli terminal çıktısı + ilerleme çubuğu
```

`core.SourceProvider` ve `core.FormatAdapter` arayüzleri sayesinde
`sources/*` ve `adapters/*` birbirinden bağımsızdır; `manager` paketi
ikisini `core` üzerinden birbirine bağlar. Yeni bir kaynak ya da format
eklemek, ilgili arayüzü uygulayan yeni bir paket + `manager.New()`'e üç
satırlık kayıt eklemek demektir.

## Ortak davranış

- Her kurulumdan sonra `<root>/var/lib/quark/installed/<ad>.json` altına
  birleşik metadata yazılır: `{ad, versiyon, format, kaynak, dosya listesi,
  sha256, kurulum zamanı}`.
- Dosya çakışması kontrolü (`core.Installer.CheckConflicts`) format/kaynak
  bağımsız çalışır - bir `.deb`'in kurmaya çalıştığı bir dosya, daha önce
  bir `.qpk` tarafından kurulmuşsa engellenir.
- `:r`, kayıtlı dosya listesine göre çalışır (formattan bağımsız); boşalan
  dizinler otomatik silinir, quark'ın bilmediği dosyalara dokunulmaz.
- Kurulum, dosya bazında atomiktir: her dosya önce `.quarknew` olarak
  yazılır, sonra aynı dosya sistemi içinde atomik `rename` ile yerine
  konur. Paket geneli için tam transactional rollback yoktur (bir paketin
  ortasında hata olursa o paket kısmen kurulmuş kalabilir) - dosya bazında
  atomiklik ile "yarı yazılmış dosya" riski ortadan kaldırılır.
- Bağımlılık çözümü native (`.qpk`) için tam (eksik bağımlılık kurulumu
  başarısız kılar), Arch/Debian için best-effort (çözülemeyen bağımlılık
  uyarıyla atlanır, kurulum devam eder) **ve derinlik-1 ile sınırlı**:
  yalnızca istenen paketin doğrudan bağımlılıkları çözülür, onların
  bağımlılıkları değil. Bu sınır gerçek bir canlı mirror testinde bulunan
  bir sorunu düzeltiyor - `quark :i arch:htop` sınırsız geçişli çözümle
  `htop -> libcap -> glibc -> linux-api-headers -> tzdata -> ...` şeklinde
  NeutronLinux'un kendi build script'leriyle (quark dışında) zaten kurulu
  olan temel sistem paketlerinin (glibc, pam, systemd-libs, gcc-libs...)
  peşine düşüyor ve bunları Arch'ın kendi derlemeleriyle **canlı sistemin
  üzerine yazmaya** çalışıyordu - quark'ın kendi DB'si bu paketleri hiç
  bilmediği için "eksik" sanıyordu. Derinlik sınırı, native olmayan
  kaynaklarda bunun önüne geçer; native için hâlâ tam/geçişli çözüm
  geçerlidir (spesifikasyon gereği).
- **glibc ABI uyumluluk kontrolü** (`internal/abi`): her dosya `Commit`
  edilmeden önce, ELF ise, gerektirdiği `GLIBC_x.y` sembol sürümleri
  hedefin kendi `libc.so.6`'sının sağladıklarıyla karşılaştırılır -
  eksik bir sürüm varsa kurulum **tamamen iptal edilir**, hiçbir dosya
  diske yazılmaz. Bu da gerçek bir olaydan geliyor: derinlik sınırı
  eklenmeden önce, `arch:htop`'un doğrudan bağımlılığı olan Arch'ın
  `ncurses`'ı GLIBC_2.42 gerektiriyordu, NeutronLinux'un kendi glibc'si
  ise 2.39'da kaldığı için `libncursesw.so.6` üzerine yazıldı ve zsh
  dahil ncurses kullanan her şey "version GLIBC_2.42' not found" ile
  çökmeye başladı - login sonrası shell hiç açılamıyordu. Arch/Debian
  paketleri kendi distrolarının (genelde çok daha yeni) glibc'siyle
  derlendiği için bu risk yapısal, tek seferlik bir hata değil; kontrol
  format/kaynak bağımsız çalışır (native `.qpk` dahil, aynı toolchain'le
  derlenmiş olsalar da bedavaya gelen bir güvenlik ağı).

## Testler

```sh
go test ./...
```

Kapsanan: native `.qpk` build+install round-trip, atomic install +
dosya-çakışması + uninstall, Arch `.PKGINFO`/pacman-db ayrıştırma, Debian
`control`/`Packages` ayrıştırma + tam bir sahte `.deb` üzerinden uçtan uca
kurulum, generic tarball'da FHS-köken algılama (hem "usr/..." hem düz
"bin/..." düzenleri), native index üzerinden search/find, native olmayan
kaynaklarda bağımlılık derinlik sınırının gerçekten tuttuğu (mock bir
kaynakla `app -> libA -> libB -> libC` zincirinde sadece `app` ve `libA`
kurulur), glibc ABI kontrolünün çalışan makinenin gerçek `libc.so.6`'sına
karşı hem uyumlu (kendi kendine) hem de yapay olarak eskitilmiş bir sürüm
kümesine karşı uyumsuz durumları doğru ayırt ettiği, manager seviyesinde
tam bir install→list→remove akışı, ve `.qprof` export/import: manifest
alan eşlemesi + tar round-trip (dosya, iç içe dizin, symlink), dotfile
geri yüklemede force olmadan var olanı atlama/force ile üzerine yazma, ve
manager seviyesinde uçtan uca export→import (native paketin yeniden
kurulması, zaten aynı sürümde kurulu bir paketin atlanması, ve `foreign`
kaynaklı bir paketin sessizce değil açık bir hatayla raporlanması).

Arch/Debian'ın **canlı mirror** entegrasyonu (gerçek ağ isteği gerektiren
`Sync()`) testlerde mock edilmemiştir - ayrıştırma mantığı (`parseDesc`,
`parsePackages`) sentetik verilerle test edilir, gerçek ağ testi opsiyonel
ve manueldir:

```sh
# gerçek Arch/Debian mirror'larına gerçekten bağlanıp senkronize eder
quark :u --config quark.conf.example
```

## v1 kapsamı dışı (mimaride yer açıldı, implement edilmedi)

- **AUR desteği** - `internal/sources/arch` sadece resmi pacman sync
  db'lerini konuşuyor; AUR'un RPC API'si + PKGBUILD derleme akışı ayrı bir
  kaynak modülü olarak eklenebilir.
- **RPM adaptörü + Fedora/openSUSE kaynağı** - `core.FormatAdapter` ve
  `core.SourceProvider` arayüzleri zaten format/kaynak-agnostik; yeni bir
  `internal/adapters/rpm` + `internal/sources/fedora` eklemek mevcut
  paketlere dokunmaz.
- **Rice paketleri** (`quark :i rice:X`) - `arch:`/`deb:` etiketleme
  düzenine üçüncü bir önek olarak eklenebilir.
- **Config-level rollback** - `core.Package.InstalledAt` zaten
  zaman damgası tutuyor; bir rollback mekanizması bunu ve dosya
  içeriklerinin bir önceki sürümünü (şu an tutulmuyor) gerektirir.

## Lisans

GPLv3 — bkz. [LICENSE](LICENSE). Türevler de açık kaynak kalmak zorunda.
