import * as THREE from 'three'

export interface RoutingSceneController {
  boost: () => void
  dispose: () => void
}

type RoutePulse = THREE.Mesh<THREE.SphereGeometry, THREE.MeshBasicMaterial> & {
  userData: {
    curve: THREE.CatmullRomCurve3
    speed: number
    offset: number
    seed: number
  }
}

export function createRoutingScene(
  host: HTMLElement,
  canvas: HTMLCanvasElement,
): RoutingSceneController {
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const isMobile = () => host.clientWidth <= 900
  const scene = new THREE.Scene()
  scene.background = new THREE.Color(0x070b0d)
  scene.fog = new THREE.FogExp2(0x070b0d, 0.027)

  const camera = new THREE.PerspectiveCamera(36, 1, 0.1, 100)
  const renderer = new THREE.WebGLRenderer({
    canvas,
    antialias: true,
    alpha: false,
    powerPreference: 'high-performance',
  })
  renderer.outputColorSpace = THREE.SRGBColorSpace
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.08

  const colors = {
    silver: 0xbac9c7,
    graphite: 0x11191b,
    graphiteSoft: 0x213033,
    teal: 0x37dec4,
    tealDeep: 0x0b8074,
    mint: 0x8ff8e7,
    gold: 0xf0b956,
  }

  const metalSilver = new THREE.MeshStandardMaterial({
    color: colors.silver,
    metalness: 0.93,
    roughness: 0.2,
  })
  const metalDark = new THREE.MeshStandardMaterial({
    color: colors.graphiteSoft,
    metalness: 0.88,
    roughness: 0.25,
  })
  const metalBlack = new THREE.MeshStandardMaterial({
    color: colors.graphite,
    metalness: 0.72,
    roughness: 0.34,
  })
  const metalGold = new THREE.MeshStandardMaterial({
    color: colors.gold,
    metalness: 0.82,
    roughness: 0.24,
    emissive: 0x2b1600,
    emissiveIntensity: 0.22,
  })
  const opticalGlass = new THREE.MeshPhysicalMaterial({
    color: 0x54d6c2,
    metalness: 0.05,
    roughness: 0.08,
    transmission: isMobile() ? 0.28 : 0.62,
    transparent: true,
    opacity: 0.82,
    thickness: 1.25,
    ior: 1.48,
    clearcoat: 1,
    clearcoatRoughness: 0.08,
    side: THREE.DoubleSide,
  })
  const coreMaterial = new THREE.MeshStandardMaterial({
    color: colors.mint,
    emissive: colors.teal,
    emissiveIntensity: 2.1,
    metalness: 0.16,
    roughness: 0.12,
  })

  function glowMaterial(color: number, opacity: number) {
    return new THREE.MeshBasicMaterial({
      color,
      transparent: true,
      opacity,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      toneMapped: false,
    })
  }

  scene.add(new THREE.HemisphereLight(0xcffcf4, 0x071011, 0.76))

  const keyLight = new THREE.DirectionalLight(0xffffff, 3.8)
  keyLight.position.set(-2, 7, 8)
  scene.add(keyLight)

  const tealLight = new THREE.PointLight(colors.teal, 11, 15, 1.7)
  tealLight.position.set(3.1, 0.5, 4.1)
  scene.add(tealLight)

  const goldLight = new THREE.PointLight(colors.gold, 5.5, 11, 2)
  goldLight.position.set(4.5, -1.6, 3.2)
  scene.add(goldLight)

  function createTube(curve: THREE.CatmullRomCurve3, color: number, radius: number, glow: number) {
    const group = new THREE.Group()
    group.add(
      new THREE.Mesh(
        new THREE.TubeGeometry(curve, isMobile() ? 48 : 96, glow, 8, false),
        glowMaterial(color, 0.11),
      ),
      new THREE.Mesh(
        new THREE.TubeGeometry(curve, isMobile() ? 48 : 96, radius, 9, false),
        glowMaterial(color, 0.94),
      ),
    )
    return group
  }

  function createArc(
    radius: number,
    tube: number,
    start: number,
    length: number,
    material: THREE.Material,
    z = 0,
  ) {
    const arc = new THREE.Mesh(new THREE.TorusGeometry(radius, tube, 14, 88, length), material)
    arc.rotation.z = start
    arc.position.z = z
    return arc
  }

  function createBlade(angle: number, radius: number, index: number) {
    const shape = new THREE.Shape()
    shape.moveTo(-0.86, -0.16)
    shape.lineTo(0.5, -0.11)
    shape.lineTo(0.82, 0.14)
    shape.lineTo(-0.54, 0.29)
    shape.closePath()
    const material = index % 3 === 0 ? metalSilver : index % 3 === 1 ? metalBlack : metalDark
    const blade = new THREE.Mesh(
      new THREE.ExtrudeGeometry(shape, {
        depth: 0.105,
        bevelEnabled: true,
        bevelSize: 0.032,
        bevelThickness: 0.032,
        bevelSegments: 2,
      }),
      material,
    )
    blade.position.set(
      Math.cos(angle) * radius,
      Math.sin(angle) * radius,
      Math.sin(angle * 2) * 0.12,
    )
    blade.rotation.z = angle + Math.PI * 0.64
    blade.rotation.x = Math.sin(angle) * 0.08
    return blade
  }

  const engine = new THREE.Group()
  engine.position.set(2.45, 0.15, 0)
  engine.rotation.set(-0.07, -0.18, 0)
  scene.add(engine)

  const outerArcs = new THREE.Group()
  const outerPattern: Array<[number, number, THREE.Material]> = [
    [0.03, 0.72, metalSilver],
    [0.91, 0.86, metalDark],
    [1.96, 0.62, metalSilver],
    [2.77, 0.95, metalBlack],
    [3.92, 0.74, metalSilver],
    [4.84, 1.03, metalDark],
  ]
  outerPattern.forEach(([start, length, material], index) => {
    outerArcs.add(
      createArc(2.42, index % 2 === 0 ? 0.105 : 0.075, start, length, material, -0.12),
    )
  })
  engine.add(outerArcs)

  const middleArcs = new THREE.Group()
  middleArcs.add(createArc(1.82, 0.07, 0.2, 1.35, metalDark, 0.18))
  middleArcs.add(createArc(1.82, 0.07, 1.82, 1.12, metalSilver, 0.18))
  middleArcs.add(createArc(1.82, 0.07, 3.24, 1.45, metalDark, 0.18))
  middleArcs.add(createArc(1.82, 0.07, 5, 0.98, metalGold, 0.18))
  engine.add(middleArcs)

  const bladeGroup = new THREE.Group()
  for (let index = 0; index < 11; index += 1) {
    bladeGroup.add(createBlade((index / 11) * Math.PI * 2, 1.54, index))
  }
  engine.add(bladeGroup)

  const innerFrame = new THREE.Mesh(new THREE.TorusGeometry(1.08, 0.07, 14, 96), metalDark)
  innerFrame.position.z = 0.34
  engine.add(innerFrame)

  const glassShell = new THREE.Mesh(new THREE.IcosahedronGeometry(0.78, 2), opticalGlass)
  glassShell.scale.z = 0.8
  glassShell.position.z = 0.42
  engine.add(glassShell)

  const core = new THREE.Mesh(new THREE.OctahedronGeometry(0.3, 1), coreMaterial)
  core.position.z = 0.5
  engine.add(core)

  const coreHalo = new THREE.Mesh(
    new THREE.SphereGeometry(0.64, 32, 24),
    glowMaterial(colors.teal, 0.095),
  )
  coreHalo.position.z = 0.38
  engine.add(coreHalo)

  const radialRails = new THREE.Group()
  for (let index = 0; index < 8; index += 1) {
    const angle = (index / 8) * Math.PI * 2 + 0.15
    const rail = new THREE.Mesh(
      new THREE.BoxGeometry(0.045, 1.28, 0.045),
      index === 6 ? metalGold : metalDark,
    )
    rail.position.set(Math.cos(angle) * 0.98, Math.sin(angle) * 0.98, 0.28)
    rail.rotation.z = angle - Math.PI / 2
    radialRails.add(rail)
  }
  engine.add(radialRails)

  const pulses: RoutePulse[] = []

  function registerRoute(
    points: THREE.Vector3[],
    color: number,
    speed: number,
    pulseCount: number,
    incoming = false,
  ) {
    const curve = new THREE.CatmullRomCurve3(points)
    scene.add(createTube(curve, color, incoming ? 0.041 : 0.026, incoming ? 0.13 : 0.085))

    for (let index = 0; index < pulseCount; index += 1) {
      const pulse = new THREE.Mesh(
        new THREE.SphereGeometry(incoming ? 0.075 : 0.055, 18, 14),
        glowMaterial(color, 1),
      ) as RoutePulse
      pulse.userData = {
        curve,
        speed,
        offset: index / pulseCount,
        seed: index * 0.37,
      }
      pulses.push(pulse)
      scene.add(pulse)
    }
  }

  registerRoute(
    [
      new THREE.Vector3(-0.75, -0.02, 0.72),
      new THREE.Vector3(0.15, 0.01, 0.76),
      new THREE.Vector3(1.15, 0.08, 0.8),
      new THREE.Vector3(2.43, 0.15, 0.82),
    ],
    colors.mint,
    0.24,
    4,
    true,
  )

  const outputs = [
    { y: 2.22, z: 0.42, color: colors.teal, speed: 0.17 },
    { y: 1.24, z: 0.62, color: colors.mint, speed: 0.21 },
    { y: 0.3, z: 0.84, color: colors.tealDeep, speed: 0.19 },
    { y: -0.76, z: 0.74, color: colors.gold, speed: 0.15 },
    { y: -1.7, z: 0.54, color: 0x62ddcb, speed: 0.2 },
  ]

  outputs.forEach((output, index) => {
    registerRoute(
      [
        new THREE.Vector3(2.46, 0.15, 0.84),
        new THREE.Vector3(2.98, output.y * 0.22, 0.85),
        new THREE.Vector3(3.88, output.y * 0.58, output.z),
        new THREE.Vector3(5.72, output.y, output.z - 0.12),
      ],
      output.color,
      output.speed,
      isMobile() ? 2 : 3,
    )

    const gate = new THREE.Mesh(
      new THREE.TorusGeometry(0.14, 0.025, 8, 28),
      index === 3 ? metalGold : metalSilver,
    )
    gate.position.set(3.64, output.y * 0.52, output.z + 0.04)
    gate.rotation.y = Math.PI / 2
    scene.add(gate)
  })

  const latticeCount = isMobile() ? 44 : 120
  const latticePositions = new Float32Array(latticeCount * 3)
  for (let index = 0; index < latticeCount; index += 1) {
    latticePositions[index * 3] = 0.4 + Math.random() * 7
    latticePositions[index * 3 + 1] = -4 + Math.random() * 8
    latticePositions[index * 3 + 2] = -2.2 - Math.random() * 3.8
  }
  const latticeGeometry = new THREE.BufferGeometry()
  latticeGeometry.setAttribute('position', new THREE.BufferAttribute(latticePositions, 3))
  const lattice = new THREE.Points(
    latticeGeometry,
    new THREE.PointsMaterial({
      color: colors.teal,
      size: isMobile() ? 0.032 : 0.045,
      transparent: true,
      opacity: 0.34,
      blending: THREE.AdditiveBlending,
      depthWrite: false,
      sizeAttenuation: true,
    }),
  )
  scene.add(lattice)

  const floor = new THREE.Mesh(
    new THREE.PlaneGeometry(25, 18),
    new THREE.MeshStandardMaterial({
      color: 0x080f11,
      metalness: 0.45,
      roughness: 0.7,
    }),
  )
  floor.rotation.x = -Math.PI / 2
  floor.position.set(1.4, -3.02, -0.6)
  scene.add(floor)

  const floorRing = new THREE.Mesh(
    new THREE.RingGeometry(1.5, 2.65, 96),
    new THREE.MeshBasicMaterial({
      color: colors.teal,
      transparent: true,
      opacity: 0.07,
      side: THREE.DoubleSide,
    }),
  )
  floorRing.rotation.x = -Math.PI / 2
  floorRing.position.set(2.45, -2.96, -0.2)
  scene.add(floorRing)

  const pointer = new THREE.Vector2()
  const pointerTarget = new THREE.Vector2()
  let routeBoost = 0
  let disposed = false
  let frameId = 0
  let elapsed = 0
  let lastTime = performance.now()

  function resize() {
    if (disposed) return
    const width = Math.max(host.clientWidth, 1)
    const height = Math.max(host.clientHeight, 1)
    const mobile = width <= 900
    camera.aspect = width / height
    if (mobile) {
      camera.fov = 43
      camera.position.set(2.25, 0.35, 18.5)
      camera.lookAt(2.25, 1.85, 0)
    } else {
      camera.fov = 36
      camera.position.set(0, 0.25, 15.7)
      camera.lookAt(0.7, 0.1, 0)
    }
    camera.updateProjectionMatrix()
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, mobile ? 1.15 : 1.65))
    renderer.setSize(width, height, false)
  }

  function handlePointerMove(event: PointerEvent) {
    const bounds = host.getBoundingClientRect()
    pointerTarget.x = ((event.clientX - bounds.left) / Math.max(bounds.width, 1)) * 2 - 1
    pointerTarget.y = ((event.clientY - bounds.top) / Math.max(bounds.height, 1)) * 2 - 1
  }

  function boost() {
    routeBoost = 1
  }

  function render(now: number) {
    if (disposed) return
    const timestamp = Number.isFinite(now) ? now : performance.now()
    const rawDelta = (timestamp - lastTime) / 1000
    const delta = Number.isFinite(rawDelta) ? Math.min(Math.max(rawDelta, 0), 0.05) : 0
    lastTime = timestamp
    elapsed += reducedMotion ? delta * 0.05 : delta
    routeBoost = Math.max(0, routeBoost - delta * 0.7)
    const speedScale = 1 + routeBoost * 2.4

    pointer.lerp(pointerTarget, 0.045)
    engine.rotation.y = -0.18 + pointer.x * 0.095
    engine.rotation.x = -0.07 - pointer.y * 0.055
    engine.position.y = 0.15 + Math.sin(elapsed * 0.72) * 0.045
    outerArcs.rotation.z = elapsed * 0.095 * speedScale
    middleArcs.rotation.z = -elapsed * 0.14 * speedScale
    bladeGroup.rotation.z = elapsed * 0.045 * speedScale
    radialRails.rotation.z = -elapsed * 0.065 * speedScale
    glassShell.rotation.x = elapsed * 0.22
    glassShell.rotation.y = elapsed * 0.28
    core.rotation.x = elapsed * 0.72
    core.rotation.y = elapsed * 0.58

    const cycle = (Math.sin(elapsed * 2.1) + 1) * 0.5
    core.scale.setScalar(1 + cycle * 0.09 + routeBoost * 0.15)
    coreHalo.scale.setScalar(0.95 + cycle * 0.22 + routeBoost * 0.3)
    coreMaterial.emissiveIntensity = 1.8 + cycle * 1.35 + routeBoost * 2.2
    tealLight.intensity = 9 + cycle * 4 + routeBoost * 7

    pulses.forEach((pulse) => {
      const { curve, speed, offset, seed } = pulse.userData
      if (curve.points.length < 2) return
      const rawProgress = elapsed * speed * speedScale + offset + seed * 0.03
      const progress = Number.isFinite(rawProgress)
        ? ((rawProgress % 1) + 1) % 1
        : offset
      const point = curve.getPoint(Math.min(progress, 0.999999))
      pulse.position.copy(point)
      const fade = Math.sin(progress * Math.PI)
      pulse.scale.setScalar(0.55 + fade * 0.85 + routeBoost * 0.2)
      pulse.material.opacity = 0.25 + fade * 0.75
    })

    lattice.rotation.z = elapsed * 0.008
    lattice.position.y = Math.sin(elapsed * 0.18) * 0.08
    floorRing.material.opacity = 0.045 + cycle * 0.04
    renderer.render(scene, camera)
    frameId = window.requestAnimationFrame(render)
  }

  function handleVisibilityChange() {
    if (document.hidden) {
      window.cancelAnimationFrame(frameId)
      return
    }
    lastTime = performance.now()
    frameId = window.requestAnimationFrame(render)
  }

  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(host)
  host.addEventListener('pointermove', handlePointerMove)
  host.addEventListener('pointerdown', boost)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  resize()
  frameId = window.requestAnimationFrame(render)

  return {
    boost,
    dispose() {
      if (disposed) return
      disposed = true
      window.cancelAnimationFrame(frameId)
      resizeObserver.disconnect()
      host.removeEventListener('pointermove', handlePointerMove)
      host.removeEventListener('pointerdown', boost)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      scene.traverse((object) => {
        if (!(object instanceof THREE.Mesh || object instanceof THREE.Points)) return
        object.geometry.dispose()
        const materials = Array.isArray(object.material) ? object.material : [object.material]
        materials.forEach((material) => material.dispose())
      })
      renderer.dispose()
      renderer.forceContextLoss()
    },
  }
}
