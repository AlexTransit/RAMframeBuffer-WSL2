# RAM framebuffer `/dev/fb0` для WSL2

Инструкция по сборке собственного ядра WSL2 с простым RAM framebuffer `/dev/fb0`.

Framebuffer:

* размер: `240x320`
* формат: `RGB565`
* глубина: `16 bpp`
* размер буфера: `153600` байт
* устройство: `/dev/fb0`
* драйвер: `vmcfb`

Это позволяет запускать в WSL2 программы, которые работают напрямую с Linux framebuffer, например код дисплея от vending/embedded устройства.

## 1. Требования

Нужны:

* Windows 10/11 с WSL2
* Ubuntu/Debian внутри WSL2
* установленный `git`
* несколько GB свободного места
* возможность собрать Linux kernel

Проверить текущий kernel:

```bash
uname -r
```

Проверить, что используется WSL2:

```bash
uname -a
```

---

# 2. Скачать исходники ядра WSL2

Используем официальный репозиторий Microsoft:

[WSL2-Linux-Kernel](https://github.com/microsoft/WSL2-Linux-Kernel?utm_source=chatgpt.com)

В данном варианте используется ветка:

```text
linux-msft-wsl-6.18.y
```

Microsoft в настоящее время имеет отдельный релиз `linux-msft-wsl-6.18.40.1`, соответствующий ядру `6.18.40.1`.

Скачать:

```bash
cd ~
git clone --depth=1 -b linux-msft-wsl-6.18.y \
    https://github.com/microsoft/WSL2-Linux-Kernel.git
```

Получится каталог:

```text
~/WSL2-Linux-Kernel
```

Перейти в него:

```bash
cd ~/WSL2-Linux-Kernel
```

Проверить версию:

```bash
make -s kernelrelease
```

Ожидается примерно:

```text
6.18.40.1-microsoft-standard-WSL2+
```

---

# 3. Установить зависимости

Для Ubuntu/Debian:

```bash
sudo apt update
sudo apt install -y \
    build-essential \
    flex \
    bison \
    dwarves \
    libssl-dev \
    libelf-dev \
    bc \
    python3 \
    pahole \
    cpio
```

Официальная инструкция Microsoft также использует эти пакеты для сборки WSL kernel.

---

# 4. Добавить драйвер `vmcfb`

Создать каталог:

```bash
mkdir -p drivers/video/fbdev/vmcfb
```

Нужно добавить три файла.

## 4.1 `drivers/video/fbdev/vmcfb/Kconfig`

Создать файл:

```text
drivers/video/fbdev/vmcfb/Kconfig
```

Содержимое:

```text
config FB_VMCFB
        tristate "WSL virtual RAM framebuffer"
        depends on FB
        select FB_CFB_FILLRECT
        select FB_CFB_COPYAREA
        select FB_CFB_IMAGEBLIT
        help
          Minimal 240x320 16-bit RGB565 RAM framebuffer for WSL.
```

---

## 4.2 `drivers/video/fbdev/vmcfb/Makefile`

Создать:

```text
drivers/video/fbdev/vmcfb/Makefile
```

Содержимое:

```make
obj-$(CONFIG_FB_VMCFB) += vmcfb.o
```

---

## 4.3 `drivers/video/fbdev/vmcfb/vmcfb.c`

Создать:

```text
drivers/video/fbdev/vmcfb/vmcfb.c
```

Содержимое:

```c
// SPDX-License-Identifier: GPL-2.0

#include <linux/fb.h>
#include <linux/init.h>
#include <linux/module.h>
#include <linux/platform_device.h>
#include <linux/vmalloc.h>
#include <linux/uaccess.h>

#define VMCFB_WIDTH  240
#define VMCFB_HEIGHT 320
#define VMCFB_BPP    16
#define VMCFB_SIZE   (VMCFB_WIDTH * VMCFB_HEIGHT * 2)

static void *vmcfb_memory;

static const struct fb_fix_screeninfo vmcfb_fix = {
	.id		= "vmcfb",
	.type		= FB_TYPE_PACKED_PIXELS,
	.visual		= FB_VISUAL_TRUECOLOR,
	.xpanstep	= 0,
	.ypanstep	= 0,
	.ywrapstep	= 0,
	.accel		= FB_ACCEL_NONE,
};

static int vmcfb_check_var(struct fb_var_screeninfo *var,
			   struct fb_info *info)
{
	if (var->xres != VMCFB_WIDTH ||
	    var->yres != VMCFB_HEIGHT ||
	    var->xres_virtual != VMCFB_WIDTH ||
	    var->yres_virtual != VMCFB_HEIGHT)
		return -EINVAL;

	var->bits_per_pixel = VMCFB_BPP;

	var->red.offset = 11;
	var->red.length = 5;

	var->green.offset = 5;
	var->green.length = 6;

	var->blue.offset = 0;
	var->blue.length = 5;

	var->transp.offset = 0;
	var->transp.length = 0;

	return 0;
}

static int vmcfb_set_par(struct fb_info *info)
{
	info->fix.visual = FB_VISUAL_TRUECOLOR;
	info->fix.line_length = VMCFB_WIDTH * 2;

	return 0;
}

static int vmcfb_mmap(struct fb_info *info,
		      struct vm_area_struct *vma)
{
	vma->vm_page_prot = pgprot_decrypted(vma->vm_page_prot);

	return remap_vmalloc_range(vma, info->screen_buffer, vma->vm_pgoff);
}

static const struct fb_ops vmcfb_ops = {
	.owner = THIS_MODULE,

	__FB_DEFAULT_SYSMEM_OPS_RDWR,

	.fb_check_var = vmcfb_check_var,
	.fb_set_par = vmcfb_set_par,

	__FB_DEFAULT_SYSMEM_OPS_DRAW,

	.fb_mmap = vmcfb_mmap,
};

static int vmcfb_probe(struct platform_device *pdev)
{
	struct fb_info *info;
	int ret;

	vmcfb_memory = vmalloc_32_user(VMCFB_SIZE);
	if (!vmcfb_memory)
		return -ENOMEM;

	memset(vmcfb_memory, 0, VMCFB_SIZE);

	info = framebuffer_alloc(sizeof(u32) * 256, &pdev->dev);
	if (!info) {
		vfree(vmcfb_memory);
		vmcfb_memory = NULL;
		return -ENOMEM;
	}

	info->screen_buffer = vmcfb_memory;
	info->screen_size = VMCFB_SIZE;
	info->fbops = &vmcfb_ops;

	info->fix = vmcfb_fix;
	info->fix.smem_start = (unsigned long)vmcfb_memory;
	info->fix.smem_len = VMCFB_SIZE;
	info->fix.line_length = VMCFB_WIDTH * 2;

	info->var.xres = VMCFB_WIDTH;
	info->var.yres = VMCFB_HEIGHT;
	info->var.xres_virtual = VMCFB_WIDTH;
	info->var.yres_virtual = VMCFB_HEIGHT;
	info->var.bits_per_pixel = VMCFB_BPP;
	info->var.activate = FB_ACTIVATE_NOW;

	ret = vmcfb_check_var(&info->var, info);
	if (ret)
		goto err_release;

	ret = vmcfb_set_par(info);
	if (ret)
		goto err_release;

	platform_set_drvdata(pdev, info);

	ret = register_framebuffer(info);
	if (ret)
		goto err_release;

	dev_info(&pdev->dev,
		 "vmcfb: %dx%d %dbpp framebuffer registered\n",
		 VMCFB_WIDTH, VMCFB_HEIGHT, VMCFB_BPP);

	return 0;

err_release:
	framebuffer_release(info);
	vfree(vmcfb_memory);
	vmcfb_memory = NULL;

	return ret;
}

static void vmcfb_remove(struct platform_device *pdev)
{
	struct fb_info *info = platform_get_drvdata(pdev);

	unregister_framebuffer(info);
	framebuffer_release(info);

	vfree(vmcfb_memory);
	vmcfb_memory = NULL;
}

static struct platform_driver vmcfb_driver = {
	.driver = {
		.name = "vmcfb",
	},
	.probe = vmcfb_probe,
	.remove = vmcfb_remove,
};

static struct platform_device *vmcfb_device;

static int __init vmcfb_init(void)
{
	int ret;

	vmcfb_device = platform_device_register_simple("vmcfb", -1, NULL, 0);
	if (IS_ERR(vmcfb_device))
		return PTR_ERR(vmcfb_device);

	ret = platform_driver_register(&vmcfb_driver);
	if (ret) {
		platform_device_unregister(vmcfb_device);
		vmcfb_device = NULL;
		return ret;
	}

	return 0;
}

static void __exit vmcfb_exit(void)
{
	platform_driver_unregister(&vmcfb_driver);

	if (vmcfb_device)
		platform_device_unregister(vmcfb_device);
}

module_init(vmcfb_init);
module_exit(vmcfb_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("VMC");
MODULE_DESCRIPTION("Minimal WSL RAM framebuffer");
```

---

# 5. Подключить драйвер в Kconfig

Открыть:

```text
drivers/video/fbdev/Kconfig
```

Найти конец списка framebuffer-драйверов и добавить:

```text
source "drivers/video/fbdev/vmcfb/Kconfig"
```

Например:

```text
...
source "drivers/video/fbdev/..."
source "drivers/video/fbdev/vmcfb/Kconfig"
```

Важно: файл должен находиться внутри `drivers/video/fbdev`.

---

# 6. Подключить драйвер в Makefile

Открыть:

```text
drivers/video/fbdev/Makefile
```

Добавить:

```make
obj-$(CONFIG_FB_VMCFB) += vmcfb/
```

---

# 7. Изменить конфигурацию ядра

Основная конфигурация WSL находится в:

```text
Microsoft/config-wsl
```

У официального WSL kernel также существует соответствующая конфигурация `config-wsl`; в ветке 6.18 она используется как конфигурация WSL kernel.

Открыть:

```bash
nano Microsoft/config-wsl
```

Нужно включить framebuffer и отключить сонсоль в него (иначе будет мешать курсор):

```text
CONFIG_FB=y
CONFIG_FB_VMCFB=y
CONFIG_FRAMEBUFFER_CONSOLE=n
```

Если в файле уже есть:

```text
CONFIG_FB=y
```

добавлять повторно не нужно.

`vmcfb` должен быть именно `y`, а не `m`, чтобы `/dev/fb0` появился непосредственно при загрузке ядра.

---

# 8. Проверить конфигурацию

Можно выполнить:

```bash
make olddefconfig KCONFIG_CONFIG=Microsoft/config-wsl
```

Затем проверить:

```bash
grep -E 'CONFIG_FB=|CONFIG_FB_VMCFB=' Microsoft/config-wsl
```

Должно быть:

```text
CONFIG_FB=y
CONFIG_FB_VMCFB=y
```

---

# 9. Собрать ядро

Из каталога:

```bash
cd ~/WSL2-Linux-Kernel
```

Запустить сборку:

```bash
make -j$(nproc) KCONFIG_CONFIG=Microsoft/config-wsl
```

Сборка занимает некоторое время.

После успешной сборки должен появиться:

```text
arch/x86/boot/bzImage
```

Проверить:

```bash
ls -lh arch/x86/boot/bzImage
```

---

# 10. Проверить собранное ядро

Можно проверить версию:

```bash
make -s kernelrelease
```

Ожидается:

```text
6.18.40.1-microsoft-standard-WSL2+
```

Также:

```bash
file arch/x86/boot/bzImage
```

---

# 11. Скопировать `bzImage` в Windows

Проще всего скопировать ядро в домашний каталог Windows:

```bash
cp arch/x86/boot/bzImage /mnt/c/Users/$USER/bzImage
```

Но `$USER` здесь является Linux-пользователем, поэтому этот вариант подходит только если имя Linux-пользователя совпадает с именем Windows.

Более универсальный вариант:

```bash
cp arch/x86/boot/bzImage /mnt/c/Users/<WINDOWS_USERNAME>/bzImage
```

Например:

```bash
cp arch/x86/boot/bzImage /mnt/c/Users/AlexM/bzImage
```

Или из PowerShell можно использовать:

```powershell
Copy-Item "\\wsl$\Ubuntu\home\<LINUX_USER>\WSL2-Linux-Kernel\arch\x86\boot\bzImage" "$env:USERPROFILE\bzImage"
```

Название дистрибутива `Ubuntu` при необходимости заменить на своё.

---

# 12. Если Windows не позволяет заменить `bzImage`

Иногда старый `bzImage` принадлежит процессу/WSL или Windows не позволяет его перезаписать.

В PowerShell выполнить:

```powershell
wsl --shutdown
```

После этого удалить старый файл:

```powershell
Remove-Item "$env:USERPROFILE\bzImage"
```

Если появляется ошибка доступа, открыть **PowerShell от имени администратора** и выполнить:

```powershell
Remove-Item -Force "$env:USERPROFILE\bzImage"
```

После этого снова скопировать новый `bzImage`.

---

# 13. Настроить `.wslconfig`

Файл:

```text
%USERPROFILE%\.wslconfig
```

То есть обычно:

```text
C:\Users\<WINDOWS_USERNAME>\.wslconfig
```

Создать или изменить его.

Минимальное содержимое:

```ini
[wsl2]
kernel=C:\\Users\\<WINDOWS_USERNAME>\\bzImage
```

Например:

```ini
[wsl2]
kernel=C:\\Users\\AlexM\\bzImage
```

`.wslconfig` является глобальной конфигурацией WSL2 и хранится в `%UserProfile%`; параметр `kernel` задаёт абсолютный путь к пользовательскому kernel image.

---

# 14. Перезапустить WSL

После изменения kernel обязательно остановить WSL:

```powershell
wsl --shutdown
```

Затем снова запустить WSL.

Проверить:

```bash
uname -r
```

Должно быть:

```text
6.18.40.1-microsoft-standard-WSL2+
```

---

# 15. Проверить framebuffer

Проверить наличие устройства:

```bash
ls -l /dev/fb0
```

Проверить framebuffer:

```bash
cat /proc/fb
```

Ожидается:

```text
0 vmcfb
```

Также:

```bash
cat /sys/class/graphics/fb0/name
```

Ожидается:

```text
vmcfb
```

Проверить параметры:

```bash
fbset -fb /dev/fb0
```

Ожидаемые параметры:

```text
mode "240x320"
    geometry 240 320 240 320 16
    ...
    rgba 5/11,6/5,5/0,0/0
endmode
```

---

# 16. Проверить размер framebuffer

Размер:

```bash
cat /sys/class/graphics/fb0/virtual_size
```

Ожидается:

```text
240,320
```

Информация:

```bash
cat /sys/class/graphics/fb0/modes
```

Также можно посмотреть:

```bash
cat /sys/class/graphics/fb0/name
```

---

# 17. Проверить запись в framebuffer

Framebuffer использует:

```text
240 × 320 × 2 = 153600 bytes
```

Создать тестовый файл:

```bash
dd if=/dev/urandom of=/tmp/fb-test bs=153600 count=1 status=none
```

Записать:

```bash
dd if=/tmp/fb-test of=/dev/fb0 bs=153600 status=none
```

Прочитать обратно:

```bash
dd if=/dev/fb0 of=/tmp/fb-readback bs=153600 status=none
```

Сравнить:

```bash
sha256sum /tmp/fb-test /tmp/fb-readback
```

Хэши должны совпадать.

Это проверяет, что `/dev/fb0` корректно принимает и возвращает весь framebuffer.

---

# 18. Просмотр framebuffer из WSL

Для просмотра `/dev/fb0` можно использовать SDL2.

Установить SDL2:

```bash
sudo apt install libsdl2-dev
```

Создать Go-проект:

```bash
mkdir -p ~/go-test/fbview
cd ~/go-test/fbview

go mod init fbview
go get github.com/veandco/go-sdl2/sdl
```

Создать:

```text
fbview.go
```

Программа должна:

1. читать `/dev/fb0`;
2. преобразовывать RGB565 → RGBA;
3. загружать изображение в SDL texture;
4. отображать его в окне;
5. обновлять изображение в реальном времени.

---

# 19. Что должно получиться

После запуска собственного ядра:

```bash
uname -r
```

пример:

```text
6.18.40.1-microsoft-standard-WSL2+
```

Framebuffer:

```bash
cat /proc/fb
```

```text
0 vmcfb
```

Устройство:

```bash
ls -l /dev/fb0
```

Параметры:

```text
240x320
RGB565
16 bpp
153600 bytes
```

Таким образом WSL2 получает настоящий Linux framebuffer `/dev/fb0`, но физического дисплея у него нет. Данные находятся в RAM и могут использоваться приложениями, которые работают с framebuffer.

---

# Структура изменений

В исходниках ядра Microsoft добавляются:

```text
WSL2-Linux-Kernel/
├── drivers/
│   └── video/
│       └── fbdev/
│           ├── Kconfig                 # изменить
│           ├── Makefile                # изменить
│           └── vmcfb/                  # добавить каталог
│               ├── Kconfig             # добавить
│               ├── Makefile            # добавить
│               └── vmcfb.c             # добавить
│
└── Microsoft/
    └── config-wsl                      # изменить
```

Изменения:

```text
drivers/video/fbdev/Kconfig
drivers/video/fbdev/Makefile
Microsoft/config-wsl
```

Новые файлы:

```text
drivers/video/fbdev/vmcfb/Kconfig
drivers/video/fbdev/vmcfb/Makefile
drivers/video/fbdev/vmcfb/vmcfb.c
```

Собранный kernel:

```text
arch/x86/boot/bzImage
```

---

# Возврат к штатному kernel WSL

Если нужно вернуться к штатному ядру, удалить или переименовать:

```text
%USERPROFILE%\.wslconfig
```

Либо удалить строку:

```ini
kernel=...
```

После этого:

```powershell
wsl --shutdown
```

Следующий запуск WSL будет использовать стандартное ядро Microsoft.

Проверить:

```bash
uname -r
```

---

# Важные замечания

`vmcfb` — это виртуальный RAM framebuffer. Он не создаёт физический дисплей и не использует Hyper-V GPU/framebuffer.

Он предназначен для:

* тестирования framebuffer-приложений;
* разработки Linux GUI для embedded-устройств;
* разработки программ для `/dev/fb0` непосредственно в WSL2;
* проверки кода дисплея без Orange Pi и физического LCD.

Framebuffer использует тот же формат:

```text
RGB565
240x320
```

что позволяет использовать один и тот же формат буфера для WSL2 и embedded-устройства.

## Источники

* [Microsoft WSL2-Linux-Kernel](https://github.com/microsoft/WSL2-Linux-Kernel?utm_source=chatgpt.com)
* [Microsoft WSL configuration documentation](https://learn.microsoft.com/en-us/windows/wsl/wsl-config?utm_source=chatgpt.com)
* [Microsoft — сборка собственного WSL kernel](https://learn.microsoft.com/ru-ru/community/content/wsl-user-msft-kernel-v6?utm_source=chatgpt.com)
