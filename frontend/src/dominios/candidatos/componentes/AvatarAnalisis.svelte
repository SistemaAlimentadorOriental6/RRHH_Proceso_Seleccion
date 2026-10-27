<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import { cubicOut, backOut } from "svelte/easing";
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import { VolumeHighIcon, VolumeOffIcon } from "@hugeicons/core-free-icons";
  import { createAvatar, type AvatarController } from "@bible-strong/avatar-web";
  import kirby from "../../../kirby.avatar.json";
  import { ORDEN_ETAPAS, type CandidatoEnVivo, type IdEtapa } from "../modelos/analisisEnVivo";
  import { almacenRrhh, veredictoDe } from "../servicios/almacenAnalisis.svelte";
  import { actualizar, destinoAdi, vuelo } from "../servicios/descarteAnimado.svelte";
  import { estaSilenciado, guardarSilencio, sonar, type Sonido } from "../servicios/sonidosAvatar";

  interface Propiedades {
    sesion: { activo: boolean; estado: string; candidatos: CandidatoEnVivo[]; repitiendo?: boolean };
  }
  let { sesion }: Propiedades = $props();

  // Cuerpo en el verde de la app (--vs-primary-raw en app.css) y ojos blancos.
  const definicion = { ...kirby, colors: { ...kirby.colors, body: "#4cc253", eyes: "#ffffff" } };
  const TAM = 72;

  // Qué hace, y qué piensa de vez en cuando, mientras una etapa del flujo está corriendo.
  const ANIMACION_ETAPA: Record<IdEtapa, string> = {
    perfil: "listening",
    genero: "curious",
    reintegro: "searching",
    adres: "searching",
    runt: "searching",
    simit: "searching",
    procuraduria: "suspicious",
    sarlaft: "suspicious",
    ofac: "suspicious",
    ia: "thinking",
  };
  const PENSAMIENTO_ETAPA: Record<IdEtapa, string[]> = {
    perfil: ["Leyendo el perfil y la hoja de vida…", "A ver qué dice su hoja de vida…", "Revisando sus datos básicos…"],
    genero: ["Revisando el género…", "Comprobando el requisito de género…"],
    reintegro: ["¿Ya trabajó con nosotros? Miro en Administremos…", "Revisando si es un reintegro…", "Buscando su cédula entre los empleados…"],
    adres: ["Mirando si tiene EPS activa…", "Consultando su afiliación en ADRES…", "¿Tendrá la EPS al día?"],
    runt: ["Consultando su licencia en el RUNT…", "Buscando su licencia de conducción…", "A ver qué categorías tiene en el RUNT…"],
    simit: ["Analizando si tiene multas…", "Buscando comparendos en el SIMIT…", "¿Tendrá multas pendientes?"],
    procuraduria: ["Buscando antecedentes disciplinarios…", "Mirando en la Procuraduría…"],
    sarlaft: ["Revisando la lista de la ONU…", "Comprobando listas de sanciones…"],
    ofac: ["Revisando la lista OFAC…", "Mirando la lista OFAC de EE. UU.…"],
    ia: ["La IA está leyendo su experiencia…", "Pidiéndole a la IA su opinión…", "Calificando su hoja de vida…"],
  };
  // Una vez por minuto el servidor revisa el correo y Computrabajo: Adi se despierta, mira y,
  // cuando termina, se duerme hasta la siguiente revisión.
  const GUARDIA = ["searching", "listening", "curious", "searching"];
  const PENSAMIENTO_GUARDIA = [
    "Revisando el correo…",
    "Buscando hojas de vida nuevas en Computrabajo…",
    "¿Habrá candidatos nuevos?",
    "A ver qué llegó al buzón…",
    "Mirando las vacantes una por una…",
    "Si llega alguien nuevo, me entero primero.",
    "Contando inscritos…",
  ];
  const QUEJAS_LABORALES = [
    "No me pagan lo suficiente para esto…", "Trabajo todo el día y ni una prima…", "Ni un día de vacaciones, y aquí sigo", "¿Para cuándo el aumento?",
    "Esto no estaba en mi contrato…", "Voy a pasar cuenta por las horas extra", "¿Y mi bono de fin de año? ¿Dónde está?", "Otro lunes más… y sin café",
    "Si me pagaran por clic ya sería millonaria", "Exijo un sindicato de avatares", "Revisar hojas de vida es lo mío, pero ¡páguenme!",
    "Ganando lo mismo desde el primer día…", "Ya casi cumplo un año sin aumento", "Tanto candidato y ni una propina para mí",
  ];
  const DESPERTAR = ["Revisando el correo y Computrabajo…", "¡Hora de revisar!", "Buenos días, a mirar qué hay de nuevo", "Ya desperté, voy a revisar"];
  const DORMIR = ["Todo al día… zzz", "Nada nuevo, me duermo un ratito", "Avísenme si llega algo… zzz"];
  const NUEVO = ["¡Llegó alguien nuevo!", "¡Hoja de vida nueva!", "¡Un candidato más!", "A ver quién es el nuevo…"];
  const ALGO_MALO = ["Algo no cuadra aquí…", "Uy, esto no se ve bien", "Mmm, esto me da mala espina", "¡Ay, no! Falló algo"];
  const ALERTA = ["Mmm, hay una alerta…", "Esto hay que revisarlo con lupa", "Una alerta, qué curioso…"];
  const APTO = ["¡Este pasa!", "¡Buen candidato!", "¡Cumple todo!", "Este sí, este sí", "¡Un apto más!"];
  const NO_APTO = ["Este no cumple…", "Qué lástima, no pasa", "No cumple los requisitos", "Este no… siguiente"];
  const PENDIENTE = ["No pude verificar todo…", "Me falta confirmar algo de este", "Queda pendiente de revisar"];
  const TERMINE = ["¡Terminé de revisar!", "¡Listo, todo analizado!", "¡Misión cumplida!", "¡Terminé! ¿Y mi aumento?", "Todo listo… y sin pago extra"];
  const ERROR_ANALISIS = ["Se me complicó este análisis…", "Uy, algo salió mal con el análisis", "Tuve un problema, ya lo reintento"];
  const A_DESCARTADOS = ["¡Este va para Descartados!", "Me llevo este a Descartados", "No pasó: a Descartados"];
  const A_ANALIZADOS = ["Este ya está listo: lo paso a Analizados", "Lo llevo a Analizados", "Listo, a su lugar en Analizados"];
  const LLEGUE_DESCARTADOS = ["¡Listo!", "Entregado en Descartados", "Ahí queda"];
  const LLEGUE_ANALIZADOS = ["¡Ya está en Analizados!", "Dejado en su lugar", "Aquí queda, bien acomodado"];
  const AL_TOCAR = [
    "¡Hey, cosquillas!", "Aquí sigo, vigilando", "¿Buscas a alguien?", "¡Hola!", "¿Necesitas algo?", "Estoy trabajando, ¿eh?",
    "Tengo muchas hojas de vida por revisar", "¡Qué amable pasar a saludar!", "Shhh, estoy concentrada", "Si me ves dormida, solo descanso un ratito",
    "Si vas a hablar con el jefe, dile que me suba el sueldo", "¿Vienes a pagarme lo que me deben?",
  ];
  const FRASES_ENOJO = [
    "¡Por favor, déjame trabajar!", "¡Ya, ya! Déjame trabajar, por favor", "¡Estoy trabajando! ¡Por favooor!", "¡Basta de clics, por favor!",
    "¡Así no puedo concentrarme!", "¡Déjame hacer mi trabajo!", "¡Me vas a hacer perder el hilo!",
    "¡Y encima no me pagan lo suficiente para esto!", "¡Por lo que me pagan, esto es demasiado!",
  ];
  const FRASES_ARRASTRE = [
    "¡Oye, suéltame!", "¡No me muevas!", "¡Estoy trabajando!", "¿Qué haces? ¡Bájame!", "¡Cuidado con mis ojos!", "¡Me vas a despeinar!",
    "¡Basta, esto no es un juguete!", "¡Mi lugar es allá arriba!", "¡Ay, ay, ay!", "¡Déjame en paz un momento!", "¡Qué mareo!", "¡Ya, ya, suéltame!",
    "¡Tengo hojas de vida por revisar!", "¡No me zarandees!", "¡Esto es un abuso de confianza!", "¡Suéltame o me enojo más!",
    "¡No me pagan lo suficiente para que me zarandeen!", "¡Esto no está en mi contrato!", "¡Ni con aumento me dejo mover así!",
  ];
  const FRASES_SOLTADA = [
    "¡Por fin! Ahora me regreso a mi lugar", "Gracias… ahora vuelvo a lo mío", "Mmm, ya me voy de vuelta", "Qué atrevimiento…",
    "Me voy a mi puesto, ¿eh?", "Listo, de vuelta al trabajo", "No vuelvas a hacerlo, ¿ok?", "Con permiso, que tengo trabajo",
  ];
  const FRASES_VOLVIENDO = [
    "Ya casi llego…", "Uf, qué camino…", "A ver si ahora me dejan tranquila", "Un momento, que ya llego", "Mi puesto está por aquí…",
    "Qué cansancio, todo por un arrastrón", "Casi, casi…",
  ];
  // Si la mueven demasiado se harta: crece, no se deja agarrar un rato y dice que la van a echar.
  const FRASES_HARTA = [
    "¡Ya basta! Tengo que trabajar o me van a echar", "¡Si no trabajo, me despiden! ¡Déjame!", "¡Mi jefe me va a echar por tu culpa!",
    "¡Necesito este trabajo! ¡No me muevas más!", "¡Estoy a un paso de que me echen!", "¡Ya me moviste demasiado! A trabajar",
    "¡Se acabó el recreo! Tengo hojas de vida pendientes", "¡O trabajo o me echan! ¡Suéltame ya!", "¡Me voy a quedar sin empleo por tu culpa!",
    "¡Ya me pagan poco y ahora me echan por tu culpa!", "¡Con lo poco que me pagan, no puedo darme el lujo de que me despidan!",
  ];
  const FRASES_HARTA_INSISTE = [
    "¡Que no! ¡Tengo que trabajar!", "¡No me toques, que me echan!", "¡Ya te dije que me van a despedir!", "¡Hoy no, estoy ocupada!",
    "¡Déjame, por favor, necesito este empleo!", "¡Nada de arrastrones!", "¡Shhh! Que viene el jefe", "¡Que me echan, que me echan!",
    "¡Aunque no me paguen bien, es mi trabajo!", "¡Que no, que no me pagan para jugar!",
  ];
  const FRASES_HARTA_FIN = ["Bueno… ya, a trabajar", "Uf, ya pasó. Ahora sí, a lo mío", "Espero que el jefe no haya visto nada…", "Respira, Adi, respira… a trabajar"];
  const FRASES_LLEGADA = ["Ahora sí, a trabajar", "Ahhh, mi lugar", "Aquí sigo, sin que me muevan", "Todo en orden otra vez", "Mucho mejor, gracias"];
  const REACCION_AL_TOCAR = ["laughing", "playful", "surprised", "excited"];
  const azar = <T,>(lista: T[]): T => lista[Math.floor(Math.random() * lista.length)];

  let ancla: HTMLDivElement;
  let volador: HTMLDivElement;
  let contenedor: HTMLDivElement;
  let avatar: AvatarController | null = null;

  let reaccion = $state<{ animacion: string; rojo: boolean } | null>(null);
  let seguido = $state<string | null>(null); // slug del candidato al que acompaña
  let enViaje = $state(false);
  // Llevando una tarjeta a Descartados sale de la pantalla de scroll (que lo recortaría y lo dejaría
  // por debajo del menú) y pasa a ser un elemento fijo del body, con la capa más alta de la página.
  let grande = $state(false);
  let agarrada = $state(false); // tiene el puntero encima, agarrándola
  let harta = $state(false); // la movieron demasiado: grande, enojada y sin dejarse agarrar
  let arrastrando = false; // ya la está moviendo (pasó el umbral de un simple clic)
  let volviendo = false; // la soltaron lejos de su sitio y regresa a él
  let enBody = false;
  function pasarAlBody(si: boolean) {
    if (si === enBody) return;
    enBody = si;
    const estilo = volador.style;
    if (si) {
      document.body.appendChild(volador);
      estilo.position = "fixed";
      estilo.inset = "auto";
      estilo.left = "0";
      estilo.top = "0";
      estilo.width = estilo.height = `${TAM}px`;
      estilo.zIndex = "2147483000";
    } else {
      ancla.appendChild(volador);
      estilo.position = estilo.inset = estilo.left = estilo.top = estilo.width = estilo.height = estilo.zIndex = "";
    }
  }
  let guardia = $state(GUARDIA[0]);
  let globo = $state<string | null>(null);
  let globoAlaDerecha = $state(false); // no cabe a la izquierda del avatar sin salirse de la pantalla
  let temporizador: ReturnType<typeof setTimeout> | undefined;
  let temporizadorGlobo: ReturnType<typeof setTimeout> | undefined;
  let prioridadActual = 0;
  let ultimoGlobo = 0;

  onDestroy(() => {
    clearTimeout(temporizador);
    clearTimeout(temporizadorGlobo);
  });

  let silenciado = $state(estaSilenciado());
  function alternarSonido() {
    silenciado = !silenciado;
    guardarSilencio(silenciado);
    if (!silenciado) sonar("pop", true); // confirma que ya suena (este clic también habilita el audio)
  }

  function reaccionar(animacion: string, ms: number, rojo = false, prioridad = 1, sonido?: Sonido) {
    if (untrack(() => reaccion) && prioridad < prioridadActual) return;
    if (sonido) sonar(sonido);
    prioridadActual = prioridad;
    reaccion = { animacion, rojo };
    clearTimeout(temporizador);
    temporizador = setTimeout(() => {
      reaccion = null;
      prioridadActual = 0;
    }, ms);
  }

  /** Globo de pensamiento: respeta una pausa mínima desde el anterior para no ser molesto. */
  function decir(texto: string, esperaMin = 9000, probabilidad = 1, ms = 3400) {
    const ahora = Date.now();
    if (ahora - ultimoGlobo < esperaMin || Math.random() > probabilidad) return;
    ultimoGlobo = ahora;
    sonar("pop");
    globo = texto;
    clearTimeout(temporizadorGlobo);
    temporizadorGlobo = setTimeout(() => (globo = null), ms);
  }

  const revisando = $derived(almacenRrhh.revisando);
  let somnoliento = $state(false);

  const candidatoSeguido = $derived(sesion.candidatos.find((c) => c.slug === seguido));
  const etapaActual = $derived(
    candidatoSeguido
      ? [...ORDEN_ETAPAS]
          .reverse()
          .find((e) => ["ejecutando", "reintentando"].includes(candidatoSeguido.etapas[e]?.estado ?? ""))
      : undefined,
  );

  const base = $derived.by(() => {
    if (!sesion.activo) return revisando ? guardia : somnoliento ? "drowsy" : "sleeping";
    if (sesion.estado === "conectando") return "waking";
    if (sesion.estado === "en_espera") return "bored";
    if (enViaje) return "excited";
    return etapaActual ? ANIMACION_ETAPA[etapaActual] : "thinking";
  });
  const animacion = $derived(reaccion?.animacion ?? base);

  // Despierta al empezar la revisión y, al terminar, se queda adormilado un instante antes de dormirse.
  let revisandoAntes = false;
  $effect(() => {
    const ahora = revisando;
    if (ahora === revisandoAntes) return;
    revisandoAntes = ahora;
    if (sesion.activo) return;
    if (ahora) {
      somnoliento = false;
      reaccionar("waking", 1400, false, 2, "nuevo");
      decir(azar(DESPERTAR), 0, 1, 3200);
      return;
    }
    somnoliento = true;
    decir(azar(DORMIR), 0, 0.7, 2600);
    const t = setTimeout(() => (somnoliento = false), 2800);
    return () => clearTimeout(t);
  });

  // Mientras dura la revisión cambia de actitud cada tanto y de vez en cuando dice qué mira.
  $effect(() => {
    if (sesion.activo || !revisando) return;
    const t = setInterval(() => {
      guardia = azar(GUARDIA);
      decir(azar([...PENSAMIENTO_GUARDIA, ...QUEJAS_LABORALES]), 8000, 0.5);
    }, 4000);
    return () => clearInterval(t);
  });

  // Reacciones a lo que pasa en el flujo. Se compara contra la foto anterior: lo que ya estaba al abrir
  // la pantalla (o viene del guardado) no cuenta como novedad.
  type Foto = { fase: string; etapas: Partial<Record<IdEtapa, string>> };
  let fotos: Map<string, Foto> | null = null;
  $effect(() => {
    const nuevas = new Map<string, Foto>();
    const previas = fotos;
    const activo = sesion.activo;
    // Lo que el servidor repite al unirse a un análisis ya en marcha no es una novedad: no se reacciona
    // (antes soltaba ráfagas de frases y sonidos al recargar).
    const repitiendo = sesion.repitiendo ?? false;
    for (const c of sesion.candidatos) {
      const etapas: Foto["etapas"] = {};
      for (const e of ORDEN_ETAPAS) etapas[e] = c.etapas[e]?.estado;
      nuevas.set(c.slug, { fase: c.fase, etapas });
      if (!previas || !activo || c.desdeCache || repitiendo) continue;
      const antes = previas.get(c.slug);
      if (!antes) {
        reaccionar("curious", 1200, false, 1, "nuevo");
        decir(azar(NUEVO), 12000, 0.5);
        continue;
      }
      for (const e of ORDEN_ETAPAS) {
        const ahora = etapas[e];
        if (ahora === antes.etapas[e]) continue;
        if (ahora === "error" || ahora === "rechazado") {
          reaccionar("angry", 2800, true, 5, "enojado");
          decir(azar(ALGO_MALO), 6000, 0.7);
        } else if (ahora === "alerta") {
          reaccionar("suspicious", 2200, false, 4, "alerta");
          decir(azar(ALERTA), 8000, 0.6);
        } else if (ahora === "reintentando") {
          reaccionar("confused", 1800, false, 2);
        } else if (ahora === "ejecutando") {
          decir(azar(PENSAMIENTO_ETAPA[e]), 9000, 0.55);
        } else if (ahora === "ok" && e === "ia") {
          reaccionar("proud", 2500, false, 3);
        }
      }
      if (antes.fase !== "completo" && c.fase === "completo") {
        const v = veredictoDe(c);
        if (v === "apto") {
          reaccionar("happy", 2800, false, 3, "feliz");
          decir(azar(APTO), 7000, 0.6);
        } else if (v === "no_apto") {
          reaccionar("sad", 2500, false, 3, "triste");
          decir(azar(NO_APTO), 8000, 0.5);
        } else if (v === "pendiente") {
          reaccionar("confused", 2200, false, 2);
          decir(azar(PENDIENTE), 8000, 0.6);
        }
      }
    }
    fotos = nuevas;
  });

  let estadoPrevio = sesion.estado;
  $effect(() => {
    const estado = sesion.estado;
    if (estado === estadoPrevio) return;
    estadoPrevio = estado;
    if (estado === "completado") {
      reaccionar("celebrate", 4000, false, 6, "celebrar");
      decir(azar(TERMINE), 0, 1);
    } else if (estado === "error") {
      reaccionar("angry", 4000, true, 6, "enojado");
      decir(azar(ERROR_ANALISIS), 0, 1);
    }
  });

  // Cuando alguien termina, Adi agarra su tarjeta y la lleva: enojado a "Descartados" si lo rechazaron,
  // contento a su lugar en "Analizados" si no.
  let llevando = false;
  $effect(() => {
    const activo = vuelo.activo;
    untrack(() => {
      if (activo) {
        llevando = true;
        if (vuelo.modo === "descartar") {
          reaccionar("angry", 12000, true, 7, "enojado");
          decir(azar(A_DESCARTADOS), 0, 1, 2600);
        } else {
          reaccionar("proud", 12000, false, 7, "feliz");
          decir(azar(A_ANALIZADOS), 0, 1, 2800);
        }
      } else if (llevando) {
        llevando = false;
        clearTimeout(temporizador);
        reaccion = null;
        prioridadActual = 0;
      }
    });
  });

  // Ya soltó la tarjeta en el menú: se queda ahí un momento, contento.
  $effect(() => {
    if (vuelo.fase !== "listo") return;
    untrack(() => {
      reaccionar("happy", 1200, false, 8, "feliz");
      decir(azar(vuelo.modo === "descartar" ? LLEGUE_DESCARTADOS : LLEGUE_ANALIZADOS), 0, 1, 1400);
    });
  });

  $effect(() => {
    avatar?.play(animacion);
  });

  /** Entrada con un pequeño rebote; salida más corta y hacia arriba, como una burbuja que se va. */
  function globoTransicion(_nodo: Element, { saliendo = false } = {}) {
    return {
      duration: saliendo ? 220 : 380,
      easing: saliendo ? cubicOut : backOut,
      css: (t: number, u: number) =>
        saliendo
          ? `opacity: ${t}; transform: translateY(${-10 * u}px) scale(${0.85 + 0.15 * t});`
          : `opacity: ${Math.min(1, t * 2)}; transform: translateY(${8 * u}px) scale(${0.5 + 0.5 * t});`,
    };
  }

  // Interacción: al pasar el mouse se sorprende y al tocarlo juega.
  function alEntrar() {
    reaccionar("surprised", 1100, false, 2);
  }
  // Si la molestan a clics seguidos se enoja, se pone grande y pide que la dejen trabajar.
  let clics: number[] = [];
  let temporizadorGrande: ReturnType<typeof setTimeout> | undefined;
  const CLICS_PARA_ENOJARSE = 4;
  const VENTANA_CLICS_MS = 2500;
  const DURACION_ENOJO_MS = 3800;
  onDestroy(() => clearTimeout(temporizadorGrande));

  function enojarse() {
    grande = true;
    reaccionar("angry", DURACION_ENOJO_MS, true, 9, "enojado");
    decir(azar(FRASES_ENOJO), 0, 1, DURACION_ENOJO_MS - 200);
    clearTimeout(temporizadorGrande);
    temporizadorGrande = setTimeout(() => {
      grande = false;
      clics = [];
    }, DURACION_ENOJO_MS);
  }

  // Arrastrarla la enoja: se queja mientras la mueven y, al soltarla, intenta volver a su lugar.
  let agarreDX = 0;
  let agarreDY = 0;
  let origenPuntero = { x: 0, y: 0 };
  let huboArrastre = false;
  let intervaloQueja: ReturnType<typeof setInterval> | undefined;
  let temporizadoresRegreso: ReturnType<typeof setTimeout>[] = [];
  onDestroy(() => {
    clearInterval(intervaloQueja);
    temporizadoresRegreso.forEach(clearTimeout);
  });

  function calmar() {
    clearTimeout(temporizador);
    reaccion = null;
    prioridadActual = 0;
  }

  const ARRASTRES_PARA_HARTARSE = 3; // arrastres en la ventana de abajo
  const VENTANA_ARRASTRES_MS = 30_000;
  const ARRASTRE_LARGO_MS = 9_000; // o un solo arrastre que no termina
  const DURACION_HARTA_MS = 15_000;
  let arrastres: number[] = [];
  let inicioArrastre = 0;
  let punteroId = -1;
  let temporizadorHarta: ReturnType<typeof setTimeout> | undefined;
  onDestroy(() => clearTimeout(temporizadorHarta));

  /** Demasiados movimientos: se suelta, crece y, un buen rato, no se deja agarrar. */
  function hartarse() {
    harta = true;
    arrastres = [];
    if (arrastrando || agarrada) {
      if (punteroId >= 0 && contenedor.hasPointerCapture(punteroId)) contenedor.releasePointerCapture(punteroId);
      arrastrando = false;
      agarrada = false;
      clearInterval(intervaloQueja);
      volviendo = true; // se zafa y vuelve a su sitio
      huboArrastre = true;
    }
    reaccionar("angry", DURACION_HARTA_MS, true, 10, "enojado");
    decir(azar(FRASES_HARTA), 0, 1, 4200);
    clearTimeout(temporizadorHarta);
    temporizadorHarta = setTimeout(() => {
      harta = false;
      calmar();
      decir(azar(FRASES_HARTA_FIN), 0, 1, 2800);
    }, DURACION_HARTA_MS);
    arrancar();
  }

  function alAgarrar(e: PointerEvent) {
    if (vuelo.activo || e.button !== 0) return;
    if (harta) {
      decir(azar(FRASES_HARTA_INSISTE), 0, 1, 2400); // no se deja
      return;
    }
    const r = contenedor.getBoundingClientRect();
    agarreDX = e.clientX - (r.left + r.width / 2);
    agarreDY = e.clientY - (r.top + r.height / 2);
    origenPuntero = { x: e.clientX, y: e.clientY };
    huboArrastre = false;
    agarrada = true;
    percha = PERCHAS[0]; // al soltarla vuelve a su sitio de siempre
    punteroId = e.pointerId;
    contenedor.setPointerCapture(e.pointerId);
  }

  function alArrastrar(e: PointerEvent) {
    if (!agarrada || arrastrando) return;
    if (Math.hypot(e.clientX - origenPuntero.x, e.clientY - origenPuntero.y) < 6) return; // todavía es un clic
    const ahora = Date.now();
    arrastres = [...arrastres.filter((t) => ahora - t < VENTANA_ARRASTRES_MS), ahora];
    if (arrastres.length >= ARRASTRES_PARA_HARTARSE) {
      hartarse();
      return;
    }
    inicioArrastre = ahora;
    arrastrando = true;
    huboArrastre = true;
    volviendo = false;
    temporizadoresRegreso.forEach(clearTimeout);
    reaccionar("angry", 600_000, true, 9, "enojado");
    decir(azar(FRASES_ARRASTRE), 0, 1, 2600);
    clearInterval(intervaloQueja);
    intervaloQueja = setInterval(() => decir(azar(FRASES_ARRASTRE), 0, 1, 2500), 2800);
    arrancar();
  }

  function alSoltar(e: PointerEvent) {
    if (!agarrada) return;
    agarrada = false;
    if (contenedor.hasPointerCapture(e.pointerId)) contenedor.releasePointerCapture(e.pointerId);
    if (!arrastrando) return; // fue un clic: lo atiende alTocar
    arrastrando = false;
    clearInterval(intervaloQueja);
    volviendo = true;
    reaccionar("angry", 600_000, true, 9); // sigue molesta mientras vuelve
    decir(azar(FRASES_SOLTADA), 0, 1, 2800);
    temporizadoresRegreso = [
      setTimeout(() => volviendo && decir(azar(FRASES_VOLVIENDO), 0, 1, 2200), 2600),
      setTimeout(() => volviendo && decir(azar(FRASES_VOLVIENDO), 0, 1, 2200), 5200),
    ];
    arrancar();
  }

  /** Llegó otra vez a su sitio: se calma y lo celebra. */
  function alLlegarAlSitio() {
    volviendo = false;
    temporizadoresRegreso.forEach(clearTimeout);
    if (harta) return; // sigue molesta hasta que se le pase
    calmar();
    reaccionar("happy", 1800, false, 8, "feliz");
    decir(azar(FRASES_LLEGADA), 0, 1, 2200);
  }

  function alTocar() {
    if (huboArrastre) {
      huboArrastre = false; // el clic que sigue a soltarla no cuenta
      return;
    }
    if (harta) {
      decir(azar(FRASES_HARTA_INSISTE), 0, 1, 2400);
      return;
    }
    const ahora = Date.now();
    clics = [...clics.filter((t) => ahora - t < VENTANA_CLICS_MS), ahora];
    if (grande || clics.length >= CLICS_PARA_ENOJARSE) {
      enojarse();
      return;
    }
    reaccionar(azar(REACCION_AL_TOCAR), 2200, false, 2, "boing");
    decir(azar(AL_TOCAR), 0, 1, 2400);
  }

  // --- Movimiento ---------------------------------------------------------------------------------
  // Un resorte amortiguado lo lleva hacia la esquina de la tarjeta que acompaña (o a su sitio en la
  // cabecera, donde además se inclina hacia el mouse). La tarjeta se mide en cada cuadro, así también
  // la sigue mientras se reacomoda en la grilla. El bucle se detiene cuando está quieto y sin análisis.
  let x = 0;
  let y = 0;
  let vx = 0;
  let vy = 0;
  let px = 0; // destino suavizado hacia el que tira el resorte
  let py = 0;
  let inclinacion = 0;
  let ultimoCambioViaje = 0;
  const VELOCIDAD_MAXIMA = 1500; // px/s
  let mx = 0;
  let my = 0;
  let cuadro = 0;
  let ultimo = 0;
  const reducido = typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
  const limitar = (n: number, m: number) => Math.max(-m, Math.min(m, n));

  /**
   * A quién acompaña, leído de lo que de verdad se ve en pantalla: la tarjeta que está analizando (o
   * recién terminó, mientras sigue resaltada). Si no hay ninguna pero el análisis está corriendo, se
   * asoma a la primera persona nueva en cola. Así no depende de que el estado y la pantalla coincidan.
   */
  let objetivo: HTMLElement | null = null;
  const ACTIVA = ".persona-activa, .recien-apto, .recien-no-apto, .recien-pendiente";
  function elegirObjetivo(): HTMLElement | null {
    const raiz = ancla.closest(".tarjeta-vacante");
    if (!raiz) return null;
    // Quien acaba de terminar espera a que Adi se lo lleve (a Descartados o a Analizados): primero
    // eso; sin tarjetas por llevar, va a la que se analiza.
    if (objetivo?.isConnected && objetivo.dataset.espera === "true") return objetivo;
    const esperando = raiz.querySelector<HTMLElement>('[data-espera="true"]');
    if (esperando) return (objetivo = esperando);
    if (objetivo?.isConnected && objetivo.matches(".persona-activa")) return objetivo;
    const activa = raiz.querySelector<HTMLElement>(".persona-activa");
    if (activa) return (objetivo = activa);
    if (objetivo?.isConnected && objetivo.matches(ACTIVA)) return objetivo;
    objetivo = sesion.activo ? raiz.querySelector<HTMLElement>('[data-fase="en_cola"]') : null;
    return objetivo;
  }

  // Dónde se posa sobre la tarjeta que acompaña: fx recorre el borde (0 izquierda, 1 derecha) y `abajo`
  // elige el borde de abajo (casi todo fuera de la tarjeta, para no tapar sus botones). La primera es su
  // sitio de siempre y la que usa para agarrar la tarjeta (ver kirbyInicial en descarteAnimado).
  type Percha = { fx: number; abajo: boolean };
  const PERCHAS: Percha[] = [
    { fx: 1, abajo: false },
    { fx: 0.5, abajo: false },
    { fx: 0, abajo: false },
    { fx: 1, abajo: true },
    { fx: 0.5, abajo: true },
    { fx: 0, abajo: true },
  ];
  let percha = PERCHAS[0];

  type Area = { left: number; top: number; right: number; bottom: number };
  let areaEl: Element | null = null;
  let globoEl = $state<HTMLDivElement | undefined>(); // el globo de pensamiento, si hay uno (bind:this)

  /**
   * Área que de verdad se ve (la del scroll de la página), para que ni Adi ni su globo queden cortados.
   * Se mide una vez y se recuerda hasta que cambia el tamaño o se desplaza algo: antes se medía en cada
   * cuadro de la animación.
   */
  let areaMedida: Area | null = null;
  let vigiaArea: ResizeObserver | null = null;
  function areaVisible(): Area {
    if (areaMedida) return areaMedida;
    if (!areaEl) {
      areaEl = ancla.closest(".area-principal");
      if (areaEl && typeof ResizeObserver !== "undefined") {
        vigiaArea = new ResizeObserver(() => (areaMedida = null));
        vigiaArea.observe(areaEl);
      }
    }
    const r = areaEl?.getBoundingClientRect();
    areaMedida = {
      left: Math.max(r?.left ?? 0, 0),
      top: Math.max(r?.top ?? 0, 0),
      right: Math.min(r?.right ?? innerWidth, innerWidth),
      bottom: Math.min(r?.bottom ?? innerHeight, innerHeight),
    };
    return areaMedida;
  }
  $effect(() => {
    const olvidar = () => (areaMedida = null);
    addEventListener("resize", olvidar, { passive: true });
    addEventListener("scroll", olvidar, { passive: true, capture: true }); // el área puede moverse con la página
    return () => {
      removeEventListener("resize", olvidar);
      removeEventListener("scroll", olvidar, { capture: true });
      vigiaArea?.disconnect();
    };
  });

  /** Mantiene a Adi (con el tamaño que tenga ahora y su globo) dentro del área visible. */
  function dentroDelArea(p: { x: number; y: number }, a: DOMRect, r: Area): { x: number; y: number } {
    const s = harta ? 2.15 : grande ? 1.75 : 1; // crece desde el 50 % / 85 % de su caja
    const lado = (TAM * (s - 1)) / 2;
    const arriba = TAM * 0.85 * (s - 1);
    const abajo = TAM * 0.15 * (s - 1);
    const altoGlobo = globoEl ? Math.max(0, globoEl.offsetHeight - TAM * 0.42) : 0;
    const M = 8;
    const minX = r.left + M + lado - a.left;
    const maxX = Math.max(minX, r.right - M - lado - TAM - a.left);
    const minY = r.top + M + arriba + altoGlobo - a.top;
    const maxY = Math.max(minY, r.bottom - M - abajo - TAM - a.top);
    // Lejos de la pantalla (la tarjeta está scrolleada fuera) no se ancla al borde: se va con su tarjeta.
    const lejos = a.top + p.y + TAM < r.top - 120 || a.top + p.y > r.bottom + 120;
    return { x: Math.min(Math.max(p.x, minX), maxX), y: lejos ? p.y : Math.min(Math.max(p.y, minY), maxY) };
  }

  function destino(t: number, a: DOMRect, area: Area): { x: number; y: number } {
    // Agarrada: va tras el puntero, tal como la tomaron.
    if (arrastrando) return { x: mx - agarreDX - TAM / 2 - a.left, y: my - agarreDY - TAM / 2 - a.top };
    // Llevando una tarjeta a "Descartados": va hacia la entrada del menú, aunque esté fuera de su vacante.
    const llevado = destinoAdi();
    if (llevado) return { x: llevado.cx - TAM / 2 - a.left, y: llevado.cy - TAM / 2 - a.top };
    const tarjeta = elegirObjetivo();
    const slug = tarjeta?.dataset.slug ?? null;
    if (slug !== seguido) {
      seguido = slug;
      percha = PERCHAS[0]; // otra tarjeta: llega a su sitio de siempre
    }
    const flota = sesion.activo ? Math.sin(t / 650) * 3 : 0;
    if (!tarjeta) {
      // En su sitio: se asoma un poco hacia donde está el mouse.
      const cx = a.left + a.width / 2;
      const cy = a.top + a.height / 2;
      return dentroDelArea({ x: limitar((mx - cx) * 0.06, 16), y: limitar((my - cy) * 0.06, 10) + flota }, a, area);
    }
    const d = tarjeta.getBoundingClientRect();
    // Si la tarjeta espera a que la lleve, se queda en la esquina desde donde la agarra.
    const p = tarjeta.dataset.espera === "true" ? PERCHAS[0] : percha;
    const recorrido = Math.max(d.width - TAM - 28, 0);
    const izquierda = d.right - TAM - 14 - (1 - p.fx) * recorrido;
    const arriba = p.abajo ? d.bottom - TAM * 0.3 : d.top - TAM * 0.55;
    return dentroDelArea({ x: izquierda - a.left, y: arriba - a.top + flota }, a, area);
  }

  /** Libre para pasear: nada la está moviendo, llevando una tarjeta ni esperando para hacerlo. */
  function puedePasear(): boolean {
    return (
      !!objetivo?.isConnected &&
      objetivo.dataset.espera !== "true" &&
      !arrastrando && !agarrada && !volviendo && !harta && !grande && !vuelo.activo && !reducido
    );
  }

  // De vez en cuando (no siempre) cambia de lugar sobre la tarjeta que acompaña. Solo toca `percha`:
  // al agarrar, arrastrar o llevar una tarjeta, destino() ya no la mira.
  $effect(() => {
    let paseo: ReturnType<typeof setTimeout>;
    const programar = () => {
      paseo = setTimeout(() => {
        if (puedePasear() && Math.random() < 0.6) {
          const otras = PERCHAS.filter((p) => p !== percha);
          // A veces vuelve a su sitio de siempre; si no, elige otro lugar al azar.
          percha = percha !== PERCHAS[0] && Math.random() < 0.35 ? PERCHAS[0] : azar(otras);
          arrancar();
        }
        programar();
      }, 6000 + Math.random() * 8000);
    };
    programar();
    return () => clearTimeout(paseo);
  });

  function paso(t: number) {
    cuadro = 0;
    try {
      pasoSeguro(t);
    } catch (error) {
      console.error("Adi: error en el movimiento", error);
      cuadro = requestAnimationFrame(paso); // un error no debe dejarlo congelado
    }
  }

  function pasoSeguro(t: number) {
    if (arrastrando && Date.now() - inicioArrastre > ARRASTRE_LARGO_MS) hartarse(); // no para de moverla
    pasarAlBody(vuelo.activo || arrastrando || volviendo); // se hace aquí, en el bucle: es lo que mueve la tarjeta, así van siempre a la par
    const dt = Math.min((t - ultimo) / 1000 || 0.016, 0.05);
    ultimo = t;
    // Una sola medición del DOM por cuadro (antes eran de 5 a 7, y con las tarjetas animándose hacía tirones).
    const a = ancla.getBoundingClientRect();
    const area = areaVisible();
    const d = destino(t, a, area);
    if (arrastrando) {
      // Sin resorte: sigue al puntero al instante; la velocidad solo sirve para inclinarla al moverla.
      vx = lerp(vx, (d.x - x) / Math.max(dt, 0.001), 0.3);
      vy = lerp(vy, (d.y - y) / Math.max(dt, 0.001), 0.3);
      x = px = d.x;
      y = py = d.y;
    } else if (reducido) {
      x = px = d.x;
      y = py = d.y;
      vx = vy = 0;
    } else {
      // Llevando una tarjeta el destino se sigue tal cual (la tarjeta cuelga de Adi); el resto del tiempo
      // el destino se suaviza primero, así sale y llega con aceleración suave en vez de arrancar de golpe,
      // y los saltos del destino (otra tarjeta, otra percha, límites del área) no se notan como tirones.
      if (vuelo.activo) {
        px = d.x;
        py = d.y;
      } else {
        const suavizado = 1 - Math.exp(-dt / 0.16);
        px += (d.x - px) * suavizado;
        py += (d.y - py) * suavizado;
      }
      // Resorte críticamente amortiguado: llega sin rebotar ni quedarse temblando.
      const rigidez = vuelo.activo ? 55 : 34;
      const amortiguacion = 2 * Math.sqrt(rigidez);
      vx += (rigidez * (px - x) - amortiguacion * vx) * dt;
      vy += (rigidez * (py - y) - amortiguacion * vy) * dt;
      const rapidez = Math.hypot(vx, vy);
      if (rapidez > VELOCIDAD_MAXIMA) {
        vx *= VELOCIDAD_MAXIMA / rapidez;
        vy *= VELOCIDAD_MAXIMA / rapidez;
      }
      x += vx * dt;
      y += vy * dt;
    }
    const velocidad = Math.hypot(vx, vy);
    // "Emocionado" al viajar rápido, con histéresis y un mínimo entre cambios para que no parpadee.
    if (t - ultimoCambioViaje > 450) {
      if (!enViaje && velocidad > 650) (enViaje = true), (ultimoCambioViaje = t);
      else if (enViaje && velocidad < 220) (enViaje = false), (ultimoCambioViaje = t);
    }
    volador.style.transform = enBody
      ? `translate3d(${(a.left + x).toFixed(2)}px, ${(a.top + y).toFixed(2)}px, 0)`
      : `translate3d(${x.toFixed(2)}px, ${y.toFixed(2)}px, 0)`;
    // Se inclina hacia donde va, pero suavizado: no reacciona a cada variación de velocidad.
    inclinacion += (limitar(vx * 0.014, 10) - inclinacion) * (1 - Math.exp(-dt / 0.09));
    contenedor.style.transform = `rotate(${inclinacion.toFixed(2)}deg)`;
    if (globoEl) {
      // El globo sale a la izquierda; si ahí no cabe en pantalla, sale a la derecha.
      const sinSitio = a.left + x - area.left < globoEl.offsetWidth + 16;
      if (sinSitio !== globoAlaDerecha) globoAlaDerecha = sinSitio;
    }
    const quieto = Math.abs(d.x - x) < 0.3 && Math.abs(d.y - y) < 0.3 && velocidad < 1;
    if (vuelo.activo) {
      actualizar(t, { cx: a.left + x + TAM / 2, cy: a.top + y + TAM / 2, vx, vy });
    }
    if (volviendo && quieto) alLlegarAlSitio();
    // Fuera de pantalla no se anima (cada cuadro mide el DOM); al volver a verse se retoma.
    const seguir = (sesion.activo && visible) || vuelo.activo || arrastrando || volviendo || (!quieto && visible);
    if (seguir) cuadro = requestAnimationFrame(paso);
  }

  function arrancar() {
    if (!cuadro) {
      px = x; // el destino suavizado parte de donde está
      py = y;
      ultimo = performance.now();
      cuadro = requestAnimationFrame(paso);
    }
  }

  function alMoverMouse(e: PointerEvent) {
    mx = e.clientX;
    my = e.clientY;
    if (visible) arrancar(); // solo se asoma hacia el mouse si se la ve
  }

  /** Adi está en pantalla: fuera de ella no hace falta moverla. */
  let visible = true;
  $effect(() => {
    if (typeof IntersectionObserver === "undefined") return;
    const observador = new IntersectionObserver(([entrada]) => {
      visible = entrada.isIntersecting;
      if (visible) arrancar();
    }, { rootMargin: "120px" });
    observador.observe(ancla);
    return () => observador.disconnect();
  });

  $effect(() => {
    void sesion.activo;
    void seguido;
    void vuelo.activo;
    arrancar();
  });

  $effect(() => {
    addEventListener("pointermove", alMoverMouse, { passive: true });
    // Revisa cada segundo que el bucle siga vivo: si por lo que sea se detuvo con trabajo pendiente, lo retoma.
    const vigilante = setInterval(() => {
      if (!cuadro && (vuelo.activo || (visible && (sesion.activo || ancla?.closest(".tarjeta-vacante")?.querySelector(ACTIVA))))) arrancar();
    }, 1000);
    return () => {
      removeEventListener("pointermove", alMoverMouse);
      clearInterval(vigilante);
    };
  });

  $effect(() => {
    avatar = createAvatar(contenedor, {
      definition: definicion,
      defaultAnimation: "idle",
      size: TAM,
      ariaLabel: "Adi, asistente del análisis",
    });
    // untrack: la animación se cambia en su propio efecto; si esta creación dependiera de ella, cada
    // cambio de expresión recrearía el avatar.
    avatar.play(untrack(() => animacion));
    return () => {
      cancelAnimationFrame(cuadro);
      cuadro = 0;
      volador?.remove(); // si estaba fijo en el body, no queda huérfano
      avatar?.destroy();
      avatar = null;
    };
  });
</script>

<div class="ancla" bind:this={ancla}>
  <div class="volador" bind:this={volador}>
    {#if globo}
      {#key globo}
        <div
          class="globo"
          class:grande={grande || harta}
          class:derecha={!seguido || globoAlaDerecha}
          bind:this={globoEl}
          role="status"
          in:globoTransicion
          out:globoTransicion={{ saliendo: true }}
        >
          <span class="texto-globo">{globo}</span>
        </div>
      {/key}
    {/if}
    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
    <div
      class="avatar-analisis"
      class:grande
      class:harta
      class:rojo={reaccion?.rojo}
      class:agarrada
      bind:this={contenedor}
      onpointerdown={alAgarrar}
      onpointermove={alArrastrar}
      onpointerup={alSoltar}
      onpointercancel={alSoltar}
      onpointerenter={alEntrar}
      onclick={alTocar}
    ></div>
  </div>
  <button
    type="button"
    class="boton-sonido"
    onclick={alternarSonido}
    aria-pressed={!silenciado}
    title={silenciado ? "Activar sonidos de Adi" : "Silenciar a Adi"}
  >
    <HugeiconsIcon icon={silenciado ? VolumeOffIcon : VolumeHighIcon} size={11} strokeWidth={2.2} />
  </button>
</div>

<style>
  .ancla {
    position: relative;
    flex-shrink: 0;
    width: 72px;
    height: 72px;
  }

  .volador {
    position: absolute;
    inset: 0;
    z-index: 20;
    will-change: transform;
    pointer-events: none;
  }

  .avatar-analisis {
    position: absolute;
    inset: 0;
    cursor: pointer;
    pointer-events: auto;
    will-change: transform, filter;
    filter: hue-rotate(0deg) saturate(1) drop-shadow(0 6px 10px rgba(44, 62, 80, 0.18));
    transition:
      filter 0.35s ease,
      scale 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
    transform-origin: 50% 85%;
    touch-action: none;
    user-select: none;
  }

  .avatar-analisis.agarrada {
    cursor: grabbing;
  }

  /* El dibujo (viewBox -150..150) se recortaba en los bordes de su caja de 72 px al saltar, inclinarse o
     crecer, porque un <svg> corta lo que se sale por defecto. */
  .avatar-analisis :global(.bs-avatar),
  .avatar-analisis :global(.bs-avatar svg) {
    overflow: visible;
  }

  /* Harta: más grande todavía, tiembla y no se deja agarrar. */
  .avatar-analisis.harta,
  .avatar-analisis.harta.grande {
    scale: 2.15;
    cursor: not-allowed;
    animation: temblor 0.09s linear infinite alternate;
  }

  /* Molesta: crece de golpe y tiembla un poco. */
  .avatar-analisis.grande {
    scale: 1.75;
    animation: temblor 0.09s linear infinite alternate;
  }

  @keyframes temblor {
    from {
      translate: -2px 0;
    }
    to {
      translate: 2px 0;
    }
  }

  .globo.grande {
    bottom: 100%;
    margin: 0 30px;
  }

  @media (prefers-reduced-motion: reduce) {
    .avatar-analisis.grande,
    .avatar-analisis.harta {
      animation: none;
    }
  }

  /* El verde gira a rojo, un momento, cuando algo sale mal en el flujo. */
  .avatar-analisis.rojo {
    filter: hue-rotate(-122deg) saturate(1.6) drop-shadow(0 6px 10px rgba(186, 54, 54, 0.35));
  }

  /* Globo de pensamiento a la izquierda del avatar, con su colita y dos burbujas. */
  .globo {
    position: absolute;
    right: calc(100% + 6px);
    bottom: 58%;
    width: max-content;
    max-width: 220px;
    padding: 8px 12px;
    border-radius: 16px;
    background: #ffffff;
    color: var(--color-texto-dark);
    font-size: 12px;
    font-weight: 600;
    line-height: 1.3;
    box-shadow: 0 8px 22px -8px rgba(44, 62, 80, 0.3);
    transform-origin: bottom right;
  }

  /* En su sitio de la cabecera no hay espacio a la izquierda: el globo sale hacia la derecha. */
  .globo.derecha {
    right: auto;
    left: calc(100% + 6px);
    transform-origin: bottom left;
  }

  .globo.derecha::before {
    right: auto;
    left: -9px;
  }

  .globo.derecha::after {
    right: auto;
    left: -16px;
  }

  .globo::before,
  .globo::after {
    content: "";
    position: absolute;
    background: #ffffff;
    border-radius: 50%;
    box-shadow: 0 2px 6px -2px rgba(44, 62, 80, 0.25);
  }

  .globo::before {
    right: -9px;
    bottom: -7px;
    width: 10px;
    height: 10px;
  }

  .globo::after {
    right: -16px;
    bottom: -14px;
    width: 6px;
    height: 6px;
  }

  /* El texto flota apenas mientras el globo está visible, y las burbujitas de la cola aparecen después. */
  .texto-globo {
    display: block;
    animation: flota-globo 2.4s ease-in-out infinite;
  }

  .globo::before {
    animation: sale-burbuja 0.3s 0.12s backwards cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  .globo::after {
    animation: sale-burbuja 0.3s 0.2s backwards cubic-bezier(0.34, 1.56, 0.64, 1);
  }

  @keyframes flota-globo {
    50% {
      transform: translateY(-1.5px);
    }
  }

  @keyframes sale-burbuja {
    from {
      opacity: 0;
      transform: scale(0);
    }
  }

  .boton-sonido {
    position: absolute;
    left: -4px;
    bottom: -4px;
    z-index: 21;
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border-radius: 50%;
    background: #ffffff;
    color: var(--color-texto-dark);
    box-shadow: 0 2px 8px -2px rgba(44, 62, 80, 0.3);
    cursor: pointer;
    opacity: 0.55;
    transition: opacity 0.2s ease, transform 0.2s ease;
  }

  .boton-sonido:hover,
  .boton-sonido:focus-visible {
    opacity: 1;
    transform: scale(1.12);
  }

  @media (prefers-reduced-motion: reduce) {
    .texto-globo,
    .globo::before,
    .globo::after {
      animation: none;
    }
  }
</style>
