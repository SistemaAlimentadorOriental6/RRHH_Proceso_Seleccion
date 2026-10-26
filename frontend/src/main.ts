import { mount } from 'svelte';
import './app.css';
import App from './App.svelte';

const contenedor = document.getElementById('app');

if (!contenedor) {
  throw new Error('No se encontró el contenedor raíz #app en el documento.');
}

const aplicacion = mount(App, {
  target: contenedor
});

export default aplicacion;
