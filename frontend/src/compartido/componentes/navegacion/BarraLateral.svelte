<script lang="ts">
  import { HugeiconsIcon } from "@hugeicons/svelte";
  import {
    ChevronDownIcon,
    ChevronUpIcon,
    ChevronRightIcon,
    ChevronLeftIcon,
    UserGroupIcon,
    Briefcase01Icon,
    SparklesIcon,
    UserIcon,
    Logout01Icon,
    WorkflowSquare10Icon,
  } from "@hugeicons/core-free-icons";
  import {
    GRUPOS_NAVEGACION_SIDEBAR,
    type SubItemNavegacion,
  } from "../../configuracion/navegacion.config";

  interface Propiedades {
    rutaActiva: string;
    nombreUsuario?: string;
    rolUsuario?: string;
    alSeleccionarRuta: (item: SubItemNavegacion) => void;
    alCerrarSesion: () => void;
  }

  let {
    rutaActiva,
    nombreUsuario = "Rosa Melano",
    rolUsuario = "jajajajajajaj",
    alSeleccionarRuta,
    alCerrarSesion,
  }: Propiedades = $props();

  let estaContraido = $state(false);
  let gruposAbiertos = $state<Record<string, boolean>>({
    candidatos: true,
    reclutamiento: true,
  });
  let menuUsuarioAbierto = $state(false);

  function obtenerIconoGrupo(clave: string) {
    return clave === "reclutamiento" ? Briefcase01Icon : UserGroupIcon;
  }

  function obtenerIconoSubItem(clave?: string) {
    return clave === "ia" ? SparklesIcon : WorkflowSquare10Icon;
  }

  function alternarBarraLateral(): void {
    estaContraido = !estaContraido;
  }

  function alternarGrupo(idGrupo: string): void {
    if (estaContraido) {
      estaContraido = false;
      gruposAbiertos = { [idGrupo]: true };
      return;
    }
    gruposAbiertos = {
      ...gruposAbiertos,
      [idGrupo]: !gruposAbiertos[idGrupo],
    };
  }
</script>

<aside
  class="barra-lateral"
  class:contraida={estaContraido}
  aria-label="Menú principal de navegación"
>
  <!-- Botón flotante para contraer o expandir -->
  <button
    type="button"
    class="boton-colapsar"
    onclick={alternarBarraLateral}
    title={estaContraido ? "Expandir menú lateral" : "Contraer menú lateral"}
  >
    <HugeiconsIcon
      icon={estaContraido ? ChevronRightIcon : ChevronLeftIcon}
      size={13}
      strokeWidth={2.2}
    />
  </button>

  <!-- Bloque Superior: Marca y Navegación -->
  <div class="contenedor-superior">
    <!-- Tarjeta de Marca -->
    <div class="tarjeta-marca" class:marca-contraida={estaContraido}>
      <div class="marca-izquierda">
        <div class="insignia-marca">
          <img src="/LOGOSAO6.svg" alt="Logo SAO6" class="logo-marca" />
        </div>
        {#if !estaContraido}
          <div class="textos-marca">
            <span class="titulo-sistema">SAO6</span>
            <span class="subtitulo-sistema">Selección y Contratación</span>
          </div>
        {/if}
      </div>
    </div>

    <!-- Lista de Grupos de Navegación -->
    <nav class="navegacion-grupos">
      {#each GRUPOS_NAVEGACION_SIDEBAR as grupo (grupo.id)}
        <div class="bloque-grupo">
          <button
            type="button"
            class="cabecera-grupo"
            class:activo={Boolean(gruposAbiertos[grupo.id])}
            class:centrado={estaContraido}
            onclick={() => alternarGrupo(grupo.id)}
            title={estaContraido ? grupo.titulo : ""}
          >
            <div class="grupo-etiqueta-icono">
              <span class="icono-grupo">
                <HugeiconsIcon
                  icon={obtenerIconoGrupo(grupo.icono)}
                  size={17}
                  strokeWidth={1.9}
                />
              </span>
              {#if !estaContraido}
                <span class="texto-grupo">{grupo.titulo}</span>
              {/if}
            </div>

            {#if !estaContraido}
              <span class="icono-chevron">
                <HugeiconsIcon
                  icon={gruposAbiertos[grupo.id]
                    ? ChevronUpIcon
                    : ChevronDownIcon}
                  size={13}
                  strokeWidth={2}
                />
              </span>
            {/if}
          </button>

          {#if gruposAbiertos[grupo.id] && !estaContraido}
            <div class="contenedor-submenu">
              {#each grupo.subItems as subItem (subItem.id)}
                {@const esActivo =
                  rutaActiva === subItem.ruta ||
                  rutaActiva.startsWith(`${subItem.ruta}/`)}
                <button
                  type="button"
                  class="item-submenu"
                  data-nav={subItem.id}
                  class:item-activo={esActivo}
                  onclick={() => alSeleccionarRuta(subItem)}
                >
                  <span class="icono-subitem">
                    <HugeiconsIcon
                      icon={obtenerIconoSubItem(subItem.icono)}
                      size={14}
                      strokeWidth={esActivo ? 2.2 : 1.8}
                    />
                  </span>
                  <span class="texto-subitem">{subItem.etiqueta}</span>
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </nav>
  </div>

  <!-- Bloque Inferior: Perfil del Reclutador y Cierre de Sesión -->
  <div class="contenedor-inferior">
    <hr class="divisor-inferior" />

    <div class="envoltorio-usuario">
      <button
        type="button"
        class="tarjeta-usuario"
        class:usuario-contraido={estaContraido}
        onclick={() => (menuUsuarioAbierto = !menuUsuarioAbierto)}
      >
        <div class="avatar-usuario">
          <HugeiconsIcon
            icon={UserIcon}
            size={16}
            color="#ffffff"
            strokeWidth={2}
          />
        </div>

        {#if !estaContraido}
          <div class="datos-usuario">
            <span class="nombre-usuario">{nombreUsuario}</span>
            <span class="rol-usuario">{rolUsuario}</span>
          </div>
        {/if}
      </button>

      {#if menuUsuarioAbierto}
        <div class="menu-flotante-usuario">
          <button
            type="button"
            class="opcion-cerrar-sesion"
            onclick={() => {
              menuUsuarioAbierto = false;
              alCerrarSesion();
            }}
          >
            <HugeiconsIcon icon={Logout01Icon} size={17} strokeWidth={2} />
            <span>Cerrar sesión</span>
          </button>
        </div>
      {/if}
    </div>
  </div>
</aside>

<style>
  .barra-lateral {
    position: relative;
    width: 232px;
    height: 100vh;
    max-height: 100vh;
    background-color: #ffffff;
    border-right: 1px solid var(--color-fondo-soft);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    padding: 14px;
    user-select: none;
    flex-shrink: 0;
    transition:
      width 0.3s cubic-bezier(0.4, 0, 0.2, 1),
      padding 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    z-index: 110;
  }

  .barra-lateral.contraida {
    width: 68px;
    padding: 14px 8px;
  }

  .boton-colapsar {
    position: absolute;
    top: 24px;
    right: -12px;
    width: 24px;
    height: 24px;
    border-radius: 999px;
    background: #ffffff;
    border: 1px solid #e2e8f0;
    box-shadow: 0 2px 8px rgba(15, 23, 42, 0.06);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-texto-grey);
    cursor: pointer;
    transition: var(--transicion-vuesax);
    z-index: 50;
  }

  .boton-colapsar:hover {
    border-color: var(--color-primario);
    color: var(--color-primario);
    transform: scale(1.06);
  }

  .contenedor-superior {
    display: flex;
    flex-direction: column;
    gap: 12px;
    flex: 1;
    min-height: 0;
  }

  .tarjeta-marca {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 6px;
    height: 44px;
    transition: var(--transicion-vuesax);
  }

  .tarjeta-marca.marca-contraida {
    justify-content: center;
    background-color: transparent;
    padding: 8px;
  }

  .marca-izquierda {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .insignia-marca {
    width: 32px;
    height: 32px;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .logo-marca {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .textos-marca {
    display: flex;
    flex-direction: column;
    line-height: 1.2;
    overflow: hidden;
  }

  .titulo-sistema {
    font-size: 13px;
    font-weight: 800;
    color: var(--color-texto-dark);
    letter-spacing: 0.03em;
    white-space: nowrap;
  }

  .subtitulo-sistema {
    font-size: 10px;
    font-weight: 600;
    color: var(--color-texto-grey);
    margin-top: 2px;
    white-space: nowrap;
  }

  .flecha-marca {
    color: var(--color-texto-grey);
    display: flex;
    align-items: center;
  }

  .navegacion-grupos {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
    overflow-y: auto;
  }

  .bloque-grupo {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .cabecera-grupo {
    width: 100%;
    min-height: 36px;
    padding: 7px 10px;
    border-radius: 11px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
    color: #64748b;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .cabecera-grupo.centrado {
    justify-content: center;
    padding: 7px 0;
  }

  .cabecera-grupo:hover {
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
  }

  .cabecera-grupo.activo {
    background-color: rgba(var(--vs-primary-raw), 0.1);
    color: var(--color-primario);
    font-weight: 700;
  }

  .grupo-etiqueta-icono {
    display: flex;
    align-items: center;
    gap: 9px;
    min-width: 0;
  }

  .icono-grupo,
  .icono-chevron {
    display: inline-flex;
    align-items: center;
    flex-shrink: 0;
  }

  .texto-grupo {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .contenedor-submenu {
    margin-left: 15px;
    padding-left: 10px;
    border-left: 1.5px solid #e2e8f0;
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding-top: 4px;
    padding-bottom: 4px;
  }

  .item-submenu {
    width: 100%;
    padding: 6px 9px;
    border-radius: 9px;
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 11px;
    font-weight: 500;
    color: #64748b;
    text-align: left;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .item-submenu:hover {
    background-color: var(--color-fondo-soft);
    color: var(--color-texto-dark);
  }

  .item-submenu.item-activo {
    background-color: rgba(var(--vs-primary-raw), 0.12);
    color: var(--color-primario);
    font-weight: 700;
  }

  .icono-subitem {
    display: inline-flex;
    align-items: center;
    flex-shrink: 0;
  }

  .texto-subitem {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .contenedor-inferior {
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding-top: 8px;
  }

  .divisor-inferior {
    border: none;
    border-top: 1px solid var(--color-fondo-soft);
  }

  .envoltorio-usuario {
    position: relative;
  }

  .tarjeta-usuario {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 9px;
    padding: 6px 8px;
    border-radius: 12px;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .tarjeta-usuario.usuario-contraido {
    justify-content: center;
    padding: 6px;
  }

  .tarjeta-usuario:hover {
    background-color: var(--color-fondo-soft);
  }

  .avatar-usuario {
    width: 32px;
    height: 32px;
    border-radius: 999px;
    background-color: var(--color-texto-dark);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .datos-usuario {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    line-height: 1.25;
    overflow: hidden;
  }

  .nombre-usuario {
    font-size: 11px;
    font-weight: 800;
    color: var(--color-texto-dark);
    white-space: nowrap;
  }

  .rol-usuario {
    font-size: 8.5px;
    font-weight: 700;
    color: var(--color-primario);
    white-space: nowrap;
    margin-top: 2px;
  }

  .menu-flotante-usuario {
    position: absolute;
    bottom: calc(100% + 8px);
    left: 0;
    right: 0;
    background: #ffffff;
    border-radius: 16px;
    padding: 6px;
    box-shadow:
      0 20px 40px -12px rgba(15, 23, 42, 0.18),
      0 4px 12px rgba(15, 23, 42, 0.08);
    border: 1px solid #f1f5f9;
    animation: modalIn 0.25s var(--curva-elastica);
  }

  .opcion-cerrar-sesion {
    width: 100%;
    padding: 10px 12px;
    border-radius: 11px;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 12px;
    font-weight: 600;
    color: #ef4444;
    cursor: pointer;
    transition: var(--transicion-vuesax);
  }

  .opcion-cerrar-sesion:hover {
    background-color: rgba(239, 68, 68, 0.08);
  }
</style>
