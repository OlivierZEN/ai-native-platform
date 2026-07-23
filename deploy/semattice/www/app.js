document.querySelectorAll(".copy").forEach((button) => {
  button.addEventListener("click", async () => {
    const target = document.getElementById(button.dataset.copy);
    if (!target) return;
    const original = button.textContent;
    try {
      await navigator.clipboard.writeText(target.innerText);
      button.textContent = "已复制";
    } catch {
      button.textContent = "复制失败";
    }
    window.setTimeout(() => {
      button.textContent = original;
    }, 1600);
  });
});

const matrixToggle = document.getElementById("toggle-matrix");
const capabilityDomains = [...document.querySelectorAll(".capability-domain")];

function syncMatrixToggle() {
  if (!matrixToggle || capabilityDomains.length === 0) return;
  const allOpen = capabilityDomains.every((domain) => domain.open);
  matrixToggle.setAttribute("aria-expanded", String(allOpen));
  matrixToggle.innerHTML = allOpen ? "收起全部 <span>−</span>" : "展开全部 <span>＋</span>";
}

matrixToggle?.addEventListener("click", () => {
  const shouldOpen = !capabilityDomains.every((domain) => domain.open);
  capabilityDomains.forEach((domain) => {
    domain.open = shouldOpen;
  });
  syncMatrixToggle();
});

capabilityDomains.forEach((domain) => {
  domain.addEventListener("toggle", syncMatrixToggle);
});

syncMatrixToggle();
