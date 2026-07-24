const bindCubeTilt = () => {
  const cubeZone = document.querySelector('.login-mode2__cube-zone');
  if (!cubeZone) return;

  cubeZone.addEventListener('pointermove', (event) => {
    const rect = cubeZone.getBoundingClientRect();
    const x = (event.clientX - rect.left) / rect.width - 0.5;
    const y = (event.clientY - rect.top) / rect.height - 0.5;
    cubeZone.style.setProperty('--mode2-tilt-y', `${x * 22}deg`);
    cubeZone.style.setProperty('--mode2-tilt-x', `${y * -22}deg`);
  });

  cubeZone.addEventListener('pointerleave', () => {
    cubeZone.style.setProperty('--mode2-tilt-y', '0deg');
    cubeZone.style.setProperty('--mode2-tilt-x', '0deg');
  });
};

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', bindCubeTilt, { once: true });
} else {
  bindCubeTilt();
}
