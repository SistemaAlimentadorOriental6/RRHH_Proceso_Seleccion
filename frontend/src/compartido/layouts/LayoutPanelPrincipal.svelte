<script lang="ts">
  import type { Snippet } from 'svelte';
  import BarraLateral from '../componentes/navegacion/BarraLateral.svelte';
  import EncabezadoPrincipal from '../componentes/navegacion/EncabezadoPrincipal.svelte';
  import type { MigaDePan, SubItemNavegacion } from '../configuracion/navegacion.config';

  interface Propiedades {
    rutaActiva: string;
    seccionActiva: string;
    tituloActivo: string;
    migasDePan?: MigaDePan[];
    nombreUsuario?: string;
    rolUsuario?: string;
    estadoMotor?: string;
    totalEventosLog?: number;
    alCambiarRuta: (item: SubItemNavegacion) => void;
    alNavegarMiga?: (ruta: string) => void;
    alCerrarSesion: () => void;
    children: Snippet;
  }

  let {
    rutaActiva,
    seccionActiva,
    tituloActivo,
    migasDePan = [],
    nombreUsuario = 'Especialista Selección',
    rolUsuario = 'Administrador RRHH',
    estadoMotor = 'Motor Orquestador Activo',
    totalEventosLog = 0,
    alCambiarRuta,
    alNavegarMiga,
    alCerrarSesion,
    children
  }: Propiedades = $props();
</script>

<div class="contenedor-layout">
  <BarraLateral
    {rutaActiva}
    {nombreUsuario}
    {rolUsuario}
    alSeleccionarRuta={alCambiarRuta}
    {alCerrarSesion}
  />

  <div class="area-principal">
    <EncabezadoPrincipal
      seccion={seccionActiva}
      titulo={tituloActivo}
      {migasDePan}
      {estadoMotor}
      {totalEventosLog}
      {alNavegarMiga}
    />

    <main class="contenido-vista">
      {@render children()}
    </main>
  </div>
</div>

<style>
  .contenedor-layout {
    width: 100%;
    height: 100vh;
    display: flex;
    background-color: #ffffff;
    overflow: hidden;
  }

  .area-principal {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: var(--color-fondo-soft);
    overflow-y: auto;
    min-width: 0;
  }

  .contenido-vista {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
  }
</style>
