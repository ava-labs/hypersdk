import React from 'react'
import {createRoot} from 'react-dom/client'
import { createHashRouter, RouterProvider } from 'react-router-dom'
import routes from './routes'

const container = document.getElementById('root')

const root = createRoot(container)

const router = createHashRouter(routes, {basename:'/'})

root.render(
    <React.StrictMode>
        <RouterProvider router={router}/>
    </React.StrictMode>
)
