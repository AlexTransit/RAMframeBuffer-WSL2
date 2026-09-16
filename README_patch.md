## Установка через patch

Если не хочется вручную создавать и изменять файлы, можно применить готовый patch.

### 1. Скачать исходники

```bash
cd ~
git clone --depth=1 -b linux-msft-wsl-6.18.y \
    https://github.com/microsoft/WSL2-Linux-Kernel.git

cd ~/WSL2-Linux-Kernel
```

### 2. Скачать patch

Положить файл:

```text
vmcfb.patch
```

в корень исходников ядра:

```text
~/WSL2-Linux-Kernel/vmcfb.patch
```

Например, если patch находится в Windows:

```bash
cp /mnt/c/Users/<WINDOWS_USERNAME>/Downloads/vmcfb.patch \
   ~/WSL2-Linux-Kernel/
```

### 3. Проверить patch перед применением

```bash
cd ~/WSL2-Linux-Kernel

git apply --check vmcfb.patch
```

Если команда ничего не вывела — patch можно применять.

### 4. Применить patch

```bash
git apply vmcfb.patch
```

Проверить изменения:

```bash
git status
```

Должны появиться:

```text
modified:   Microsoft/config-wsl
modified:   drivers/video/fbdev/Kconfig
modified:   drivers/video/fbdev/Makefile
```

и новые файлы:

```text
drivers/video/fbdev/vmcfb/Kconfig
drivers/video/fbdev/vmcfb/Makefile
drivers/video/fbdev/vmcfb/vmcfb.c
```

### 5. Проверить конфигурацию

```bash
grep -E 'CONFIG_FB=|CONFIG_FB_VMCFB=' Microsoft/config-wsl
```

Должно быть:

```text
CONFIG_FB=y
CONFIG_FB_VMCFB=y
```

Если всё правильно, можно переходить к сборке:

```bash
make olddefconfig KCONFIG_CONFIG=Microsoft/config-wsl
make -j$(nproc) KCONFIG_CONFIG=Microsoft/config-wsl
```

После сборки:

```text
arch/x86/boot/bzImage
```

будет готовый kernel.

---

## Если patch не применился

Сначала проверить:

```bash
git apply --check vmcfb.patch
```

Если Git сообщает:

```text
patch does not apply
```

или:

```text
patch failed
```

не нужно вручную продолжать установку.

Причина обычно в том, что версия исходников kernel отличается от версии, для которой был создан patch.

Проверить ветку:

```bash
git branch --show-current
```

Должно быть:

```text
linux-msft-wsl-6.18.y
```

Проверить состояние репозитория:

```bash
git status
```

Если в исходниках уже были изменения, можно вернуть их:

```bash
git restore .
```

После этого снова:

```bash
git apply --check vmcfb.patch
git apply vmcfb.patch
```

### Удалить patch

Сам файл patch из исходников можно удалить:

```bash
rm vmcfb.patch
```

Это не отменяет уже применённые изменения.

Чтобы полностью отменить patch:

```bash
git apply -R vmcfb.patch
```

После этого проверить:

```bash
git status
```

---

## После сборки

Скопировать kernel:

```bash
cp arch/x86/boot/bzImage /mnt/c/Users/<WINDOWS_USERNAME>/bzImage
```

Если старый `bzImage` не удаляется:

```powershell
wsl --shutdown
Remove-Item -Force "$env:USERPROFILE\bzImage"
```

Затем снова:

```bash
cp arch/x86/boot/bzImage /mnt/c/Users/<WINDOWS_USERNAME>/bzImage
```

В Windows `%USERPROFILE%\.wslconfig`:

```ini
[wsl2]
kernel=C:\\Users\\<WINDOWS_USERNAME>\\bzImage
```

Перезапустить WSL:

```powershell
wsl --shutdown
```

Проверить:

```bash
uname -r
```

И framebuffer:

```bash
cat /proc/fb
```

Ожидается:

```text
0 vmcfb
```

Параметры:

```bash
fbset -fb /dev/fb0
```

Должно быть:

```text
240x320
16 bpp
RGB565
```
