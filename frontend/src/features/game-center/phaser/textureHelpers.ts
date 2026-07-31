import Phaser from 'phaser'

export function createRectTexture(
  scene: Phaser.Scene,
  key: string,
  width: number,
  height: number,
  color: number,
  radius = 0,
): void {
  if (scene.textures.exists(key)) return
  const graphics = scene.add.graphics()
  graphics.fillStyle(color, 1)
  if (radius > 0) {
    graphics.fillRoundedRect(0, 0, width, height, radius)
  } else {
    graphics.fillRect(0, 0, width, height)
  }
  graphics.generateTexture(key, width, height)
  graphics.destroy()
}

export function createCircleTexture(scene: Phaser.Scene, key: string, diameter: number, color: number): void {
  if (scene.textures.exists(key)) return
  const graphics = scene.add.graphics()
  graphics.fillStyle(color, 1)
  graphics.fillCircle(diameter / 2, diameter / 2, diameter / 2)
  graphics.generateTexture(key, diameter, diameter)
  graphics.destroy()
}
